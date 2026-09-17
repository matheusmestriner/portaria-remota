CREATE TABLE access_authorizations (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),company_id uuid NOT NULL REFERENCES companies(id),condo_id uuid NOT NULL,
 invitation_id uuid NOT NULL,access_id uuid NOT NULL,used boolean NOT NULL DEFAULT false,expires_at timestamptz NOT NULL,
 FOREIGN KEY(company_id,condo_id) REFERENCES condos(company_id,id),FOREIGN KEY(company_id,access_id) REFERENCES records(company_id,id)
);
ALTER TABLE access_authorizations ENABLE ROW LEVEL SECURITY;
ALTER TABLE access_authorizations FORCE ROW LEVEL SECURITY;
CREATE POLICY auth_scope ON access_authorizations USING(company_id=app.company() AND app.can_condo(condo_id) AND app.setting('role')<>'resident') WITH CHECK(company_id=app.company() AND app.can_condo(condo_id) AND app.setting('role')<>'resident');
CREATE TABLE sip_accounts (id uuid PRIMARY KEY DEFAULT gen_random_uuid(),company_id uuid NOT NULL REFERENCES companies(id),subject text NOT NULL,username text NOT NULL,password_cipher text NOT NULL,active boolean NOT NULL DEFAULT true,UNIQUE(company_id,subject));
ALTER TABLE sip_accounts ENABLE ROW LEVEL SECURITY;
ALTER TABLE sip_accounts FORCE ROW LEVEL SECURITY;
CREATE POLICY sip_read ON sip_accounts FOR SELECT USING(company_id=app.company() AND (subject=app.setting('subject') OR app.setting('role') IN ('company_admin','support')));
CREATE POLICY sip_write ON sip_accounts FOR ALL USING(company_id=app.company() AND app.setting('role') IN ('company_admin','support')) WITH CHECK(company_id=app.company() AND app.setting('role') IN ('company_admin','support'));
CREATE TABLE maintenance_runs (company_id uuid PRIMARY KEY REFERENCES companies(id),last_run timestamptz NOT NULL DEFAULT now());
ALTER TABLE maintenance_runs ENABLE ROW LEVEL SECURITY;
ALTER TABLE maintenance_runs FORCE ROW LEVEL SECURITY;
CREATE POLICY maintenance_scope ON maintenance_runs USING(company_id=app.company());
ALTER TABLE tickets ADD COLUMN call_id text;
CREATE UNIQUE INDEX ticket_calls ON tickets(company_id,call_id) WHERE call_id IS NOT NULL;
GRANT SELECT,INSERT,UPDATE ON access_authorizations,sip_accounts,maintenance_runs TO portaria_app;
CREATE FUNCTION app.support_active(company uuid,actor text) RETURNS boolean LANGUAGE sql SECURITY DEFINER SET search_path=public,pg_temp AS $$ SELECT EXISTS(SELECT 1 FROM support_grants WHERE company_id=company AND subject=actor AND expires_at>now()) $$;
REVOKE ALL ON FUNCTION app.support_active(uuid,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION app.support_active(uuid,text) TO portaria_app;
