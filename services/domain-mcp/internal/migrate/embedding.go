package migrate

// EmbeddingDim es la dimensión de las columnas pgvector del esquema, fijada por la
// migración 000275 (bge-m3). Única fuente de verdad: un embedder que produzca otra
// dimensión hace fallar el INSERT, así que los binarios y los tests la leen de acá
// en vez de repetir el número.
//
// Vive en este paquete y no en internal/db porque internal/db importa
// internal/mcp/server, y eso cicla contra los tests de los servicios que la
// necesitan. Acá el paquete no tiene dependencias internas y además queda al lado
// de la migración que la fija.
const EmbeddingDim = 1024
