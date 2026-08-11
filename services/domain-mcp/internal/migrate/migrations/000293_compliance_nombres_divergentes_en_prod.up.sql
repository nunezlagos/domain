-- migration: 000293_compliance_nombres_divergentes_en_prod
-- author: nunezlagos
-- issue: DOMAINSERV-261
-- description: reconcilia los nombres de compliance que divergieron entre producción y el repo
--   cuando la 000291 se editó después de estar aplicada: una tabla, un índice y tres columnas
-- breaking: no — solo renombra; ninguna fila cambia y ningún tipo se altera
-- estimated_duration: <1s (renames son cambios de catálogo; las tablas están vacías en prod)
--
-- POR QUÉ EXISTE ESTA MIGRACIÓN
--
-- El commit f55e92dd ("el schema se nombra conforme, aprovechando que todavia se podia
-- editar") renombró objetos DENTRO de la 000291, que ya estaba aplicada en producción. El
-- archivo cambió; la base no. Resultado medido el 2026-08-11 contra la DB de prod:
--
--   schema_migrations = 292, dirty = false   <- todo parece sano
--   pero la tabla se llama framework_controls y el código pide compliance_framework_controls
--
-- Por eso domain_compliance_report devuelve 'relation "compliance_framework_controls" does
-- not exist (SQLSTATE 42P01)', y con él revienta la fase sdd-compliance de todo flow SDD
-- full. El flow full quedó IMPOSIBLE: sdd-tasks depende de sdd-compliance, así que la fase
-- no se puede incluir (revienta) ni saltear (rompe el DAG).
--
-- Es exactamente el caso que la policy "una migración ya aplicada no se edita" describe, y
-- su remedio es el que la policy indica: corregir HACIA ADELANTE. La 291 y la 292 NO se
-- tocan — editarlas es lo que produjo esta divergencia.
--
-- POR QUÉ CADA RENAME LLEVA SU PROPIO GUARD
--
-- La misma migración tiene que servir para dos bases distintas:
--   · prod, donde los objetos tienen el nombre VIEJO y hay que renombrarlos
--   · un deploy limpio, donde la 291 ya los creó con el nombre NUEVO y esto es un no-op
--
-- MEDIDO en postgres 16, porque el comportamiento no es uniforme y de ahí el DO block:
--   ALTER TABLE IF EXISTS  <ausente> RENAME TO ...  -> NOTICE, no falla
--   ALTER INDEX IF EXISTS  <ausente> RENAME TO ...  -> NOTICE, no falla
--   ALTER TABLE IF EXISTS t RENAME COLUMN <ausente> -> ERROR: column does not exist
--
-- El IF EXISTS de un ALTER TABLE aplica a la TABLA, no a la columna. Escribir los tres
-- renames de columna con esa forma habría hecho fallar la migración en todo deploy limpio.

BEGIN;

-- tabla e índice: el IF EXISTS alcanza, el propio Postgres los saltea si ya están renombrados
ALTER TABLE IF EXISTS framework_controls RENAME TO compliance_framework_controls;
ALTER INDEX IF EXISTS framework_controls_control_idx
    RENAME TO compliance_framework_controls_control_idx;

-- columnas: se consulta el catálogo porque RENAME COLUMN no admite IF EXISTS de columna
DO $$
DECLARE
    r RECORD;
BEGIN
    FOR r IN
        SELECT * FROM (VALUES
            ('project_compliance_frameworks', 'activado_por',  'activado_por_id'),
            ('project_control_status',        'evaluado_por',  'evaluado_por_id'),
            ('compliance_waivers',            'otorgado_por', 'otorgado_por_id')
        ) AS t(tabla, viejo, nuevo)
    LOOP
        -- se exige que el nombre VIEJO exista Y que el nuevo NO: así el rename no se
        -- intenta dos veces ni pisa una columna que ya tiene el nombre correcto
        IF EXISTS (
            SELECT 1 FROM information_schema.columns
            WHERE table_schema = 'public' AND table_name = r.tabla AND column_name = r.viejo
        ) AND NOT EXISTS (
            SELECT 1 FROM information_schema.columns
            WHERE table_schema = 'public' AND table_name = r.tabla AND column_name = r.nuevo
        ) THEN
            EXECUTE format('ALTER TABLE %I RENAME COLUMN %I TO %I', r.tabla, r.viejo, r.nuevo);
            RAISE NOTICE 'renombrada %.% -> %', r.tabla, r.viejo, r.nuevo;
        END IF;
    END LOOP;
END $$;

COMMIT;
