






INSERT INTO runtime_configs (key, value, description, is_hot_reloadable) VALUES
  ('metrics_enabled', 'true', 'Habilita/deshabilita endpoint de métricas Prometheus', TRUE)
ON CONFLICT (key) DO NOTHING;
