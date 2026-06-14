-- migration: seed_metrics_enabled
-- author: nunezlagos
-- issue: HU-27.3
-- description: agrega metrics_enabled al seed de runtime_configs (faltante en 000041)
-- breaking: false
-- estimated_duration: <1s

INSERT INTO runtime_configs (key, value, description, is_hot_reloadable) VALUES
  ('metrics_enabled', 'true', 'Habilita/deshabilita endpoint de métricas Prometheus', TRUE)
ON CONFLICT (key) DO NOTHING;
