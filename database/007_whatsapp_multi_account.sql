CREATE TABLE whatsapp_accounts (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 company_id uuid NOT NULL REFERENCES companies(id),
 condo_id uuid,
 bridge_key text NOT NULL,
 label text NOT NULL,
 status text NOT NULL DEFAULT 'disconnected' CHECK(status IN ('disconnected','connecting','connected','logged_out','error')),
 phone text,
 details jsonb NOT NULL DEFAULT '{}',
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 FOREIGN KEY(company_id,condo_id) REFERENCES condos(company_id,id),
 UNIQUE(company_id,bridge_key)
);

CREATE UNIQUE INDEX whatsapp_accounts_default_unique
 ON whatsapp_accounts(company_id)
 WHERE condo_id IS NULL;

CREATE UNIQUE INDEX whatsapp_accounts_condo_unique
 ON whatsapp_accounts(company_id,condo_id)
 WHERE condo_id IS NOT NULL;

ALTER TABLE whatsapp_accounts ENABLE ROW LEVEL SECURITY;
ALTER TABLE whatsapp_accounts FORCE ROW LEVEL SECURITY;
CREATE POLICY whatsapp_accounts_scope ON whatsapp_accounts
 USING(company_id=app.company() AND (condo_id IS NULL OR app.can_condo(condo_id)))
 WITH CHECK(company_id=app.company() AND (condo_id IS NULL OR app.can_condo(condo_id)));

GRANT SELECT,INSERT,UPDATE,DELETE ON whatsapp_accounts TO portaria_app;

INSERT INTO whatsapp_accounts(company_id,bridge_key,label,status,phone,details)
SELECT DISTINCT source.company_id,'default','Número padrão da revenda',
       coalesce(i.status,'disconnected'),
       nullif(i.details->>'phone',''),
       coalesce(i.details,'{}'::jsonb)
FROM (
 SELECT company_id FROM integrations WHERE kind='whatsapp'
 UNION
 SELECT company_id FROM whatsapp_messages
 UNION
 SELECT company_id FROM whatsapp_flows
) source
LEFT JOIN integrations i ON i.company_id=source.company_id AND i.kind='whatsapp'
ON CONFLICT DO NOTHING;

ALTER TABLE whatsapp_messages ADD COLUMN account_id uuid;
UPDATE whatsapp_messages m
 SET account_id=a.id
 FROM whatsapp_accounts a
 WHERE a.company_id=m.company_id AND a.condo_id IS NULL;
ALTER TABLE whatsapp_messages ALTER COLUMN account_id SET NOT NULL;
ALTER TABLE whatsapp_messages
 ADD CONSTRAINT whatsapp_messages_account_fk
 FOREIGN KEY(account_id) REFERENCES whatsapp_accounts(id);
ALTER TABLE whatsapp_messages DROP CONSTRAINT whatsapp_messages_pkey;
ALTER TABLE whatsapp_messages
 ADD PRIMARY KEY(company_id,account_id,message_id);

ALTER TABLE whatsapp_flows ADD COLUMN account_id uuid;
UPDATE whatsapp_flows f
 SET account_id=a.id
 FROM whatsapp_accounts a
 WHERE a.company_id=f.company_id AND a.condo_id IS NULL;
ALTER TABLE whatsapp_flows ALTER COLUMN account_id SET NOT NULL;
ALTER TABLE whatsapp_flows
 ADD CONSTRAINT whatsapp_flows_account_fk
 FOREIGN KEY(account_id) REFERENCES whatsapp_accounts(id);
ALTER TABLE whatsapp_flows DROP CONSTRAINT whatsapp_flows_pkey;
ALTER TABLE whatsapp_flows
 ADD PRIMARY KEY(company_id,account_id,sender);

CREATE INDEX whatsapp_accounts_scope_idx ON whatsapp_accounts(company_id,condo_id,status);
CREATE INDEX whatsapp_messages_account_idx ON whatsapp_messages(company_id,account_id,created_at DESC);
CREATE INDEX whatsapp_flows_account_idx ON whatsapp_flows(company_id,account_id,updated_at DESC);

COMMENT ON TABLE whatsapp_accounts IS 'Contas WhatsApp isoladas por revenda, com conta padrão opcional e conta específica por condomínio.';
COMMENT ON COLUMN whatsapp_accounts.bridge_key IS 'Identificador opaco usado entre API e gateway Whatsmeow; nunca é fornecido pelo navegador.';
