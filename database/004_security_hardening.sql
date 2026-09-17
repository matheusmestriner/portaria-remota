ALTER TABLE agent_keys ADD COLUMN IF NOT EXISTS expires_at timestamptz NOT NULL DEFAULT (now() + interval '90 days');
ALTER TABLE agent_keys ADD COLUMN IF NOT EXISTS rotated_at timestamptz NOT NULL DEFAULT now();

CREATE OR REPLACE FUNCTION app.resolve_agent(hash text)
RETURNS TABLE(company_id uuid,condo_id uuid,id uuid)
LANGUAGE sql SECURITY DEFINER SET search_path=public,pg_temp AS $$
 SELECT a.company_id,a.condo_id,a.id
 FROM agent_keys a JOIN companies c ON c.id=a.company_id
 WHERE a.token_hash=hash AND a.active AND a.expires_at>now() AND c.status='active'
$$;

