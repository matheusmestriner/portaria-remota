ALTER TABLE integrations
ADD COLUMN IF NOT EXISTS details jsonb NOT NULL DEFAULT '{}';

COMMENT ON COLUMN integrations.details IS 'Metadados não sensíveis da integração. Para WhatsApp, registra o número pareado em formato E.164.';
