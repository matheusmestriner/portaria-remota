CREATE ROLE portaria_app NOLOGIN NOSUPERUSER NOBYPASSRLS;
CREATE SCHEMA app;
CREATE FUNCTION app.setting(key text) RETURNS text LANGUAGE sql STABLE AS $$ SELECT nullif(current_setting('app.' || key, true), '') $$;
CREATE FUNCTION app.company() RETURNS uuid LANGUAGE sql STABLE AS $$ SELECT app.setting('company')::uuid $$;
CREATE FUNCTION app.is_platform() RETURNS boolean LANGUAGE sql STABLE AS $$ SELECT coalesce(app.setting('platform') = 'true', false) $$;
CREATE FUNCTION app.can_condo(id uuid) RETURNS boolean LANGUAGE sql STABLE AS $$
 SELECT app.setting('role') IN ('company_admin','support') OR id::text = ANY(string_to_array(app.setting('condos'), ','))
$$;
CREATE FUNCTION app.can_unit(id uuid) RETURNS boolean LANGUAGE sql STABLE AS $$
 SELECT app.setting('role') <> 'resident' OR id::text = ANY(string_to_array(app.setting('units'), ','))
$$;
CREATE TABLE companies (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), name text NOT NULL, slug text NOT NULL UNIQUE,
 status text NOT NULL DEFAULT 'draft' CHECK(status IN ('draft','active','suspended')),
 brand jsonb NOT NULL DEFAULT '{"primary":"#147d64","name":""}', created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE platform_admins (subject text PRIMARY KEY, created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE memberships (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), company_id uuid NOT NULL REFERENCES companies(id), subject text NOT NULL,
 name text NOT NULL, role text NOT NULL CHECK(role IN ('company_admin','supervisor','operator','manager','resident')),
 condo_ids uuid[] NOT NULL DEFAULT '{}', unit_ids uuid[] NOT NULL DEFAULT '{}', active boolean NOT NULL DEFAULT true,
 whatsapp text, whatsapp_verified boolean NOT NULL DEFAULT false,
 UNIQUE(company_id,subject), UNIQUE(company_id,whatsapp)
);
CREATE TABLE support_grants (id uuid PRIMARY KEY DEFAULT gen_random_uuid(), company_id uuid NOT NULL REFERENCES companies(id), subject text NOT NULL, reason text NOT NULL, expires_at timestamptz NOT NULL, created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE domains (id uuid PRIMARY KEY DEFAULT gen_random_uuid(), company_id uuid NOT NULL REFERENCES companies(id), hostname text UNIQUE NOT NULL, verification_token text NOT NULL, verified_at timestamptz, tls_checked_at timestamptz, created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE condos (id uuid PRIMARY KEY DEFAULT gen_random_uuid(), company_id uuid NOT NULL REFERENCES companies(id), name text NOT NULL, address text NOT NULL DEFAULT '', created_at timestamptz NOT NULL DEFAULT now(), UNIQUE(company_id,id));
CREATE TABLE records (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), company_id uuid NOT NULL REFERENCES companies(id), condo_id uuid NOT NULL,
 kind text NOT NULL CHECK(kind IN ('blocks','units','people','vehicles','access-points','devices','credentials','cameras','intercoms','rules','incidents')),
 unit_id uuid, data jsonb NOT NULL, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(company_id,id), FOREIGN KEY(company_id,condo_id) REFERENCES condos(company_id,id),
 FOREIGN KEY(company_id,unit_id) REFERENCES records(company_id,id)
);
CREATE TABLE invitations (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), company_id uuid NOT NULL REFERENCES companies(id), condo_id uuid NOT NULL,
 unit_id uuid NOT NULL, issuer text NOT NULL, guest_name text NOT NULL, vehicle_plate text,
 valid_from timestamptz NOT NULL, valid_until timestamptz NOT NULL, access_ids uuid[] NOT NULL,
 status text NOT NULL DEFAULT 'active' CHECK(status IN ('active','cancelled','used')),
 token_hash text UNIQUE NOT NULL, credential_hash text UNIQUE NOT NULL, credential_cipher text NOT NULL,
 request_key text NOT NULL, channel text NOT NULL CHECK(channel IN ('web','app','whatsapp')),
 created_at timestamptz NOT NULL DEFAULT now(), UNIQUE(company_id,request_key),
 CHECK(valid_until > valid_from), CHECK(cardinality(access_ids)>0),
 FOREIGN KEY(company_id,condo_id) REFERENCES condos(company_id,id), FOREIGN KEY(company_id,unit_id) REFERENCES records(company_id,id)
);
CREATE TABLE commands (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), company_id uuid NOT NULL REFERENCES companies(id), condo_id uuid NOT NULL,
 access_id uuid NOT NULL, device_id uuid NOT NULL, actor text NOT NULL, reason text NOT NULL,
 request_key text NOT NULL, state text NOT NULL DEFAULT 'pending' CHECK(state IN ('pending','sent','confirmed','failed','unknown','expired')),
 expires_at timestamptz NOT NULL, dispatched_at timestamptz, completed_at timestamptz, created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(company_id,request_key), FOREIGN KEY(company_id,condo_id) REFERENCES condos(company_id,id),
 FOREIGN KEY(company_id,access_id) REFERENCES records(company_id,id), FOREIGN KEY(company_id,device_id) REFERENCES records(company_id,id)
);
CREATE TABLE events (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), company_id uuid NOT NULL REFERENCES companies(id), condo_id uuid,
 unit_id uuid, kind text NOT NULL, actor text NOT NULL, resource_id uuid, detail jsonb NOT NULL DEFAULT '{}',
 source_key text, created_at timestamptz NOT NULL DEFAULT now(), UNIQUE(company_id,source_key),
 FOREIGN KEY(company_id,condo_id) REFERENCES condos(company_id,id)
);
CREATE TABLE tickets (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), company_id uuid NOT NULL REFERENCES companies(id), condo_id uuid NOT NULL,
 unit_id uuid, access_id uuid, title text NOT NULL, state text NOT NULL DEFAULT 'waiting' CHECK(state IN ('waiting','claimed','closed')),
 owner text, created_at timestamptz NOT NULL DEFAULT now(), closed_at timestamptz,
 FOREIGN KEY(company_id,condo_id) REFERENCES condos(company_id,id), FOREIGN KEY(company_id,unit_id) REFERENCES records(company_id,id), FOREIGN KEY(company_id,access_id) REFERENCES records(company_id,id)
);
CREATE TABLE agent_keys (id uuid PRIMARY KEY DEFAULT gen_random_uuid(), company_id uuid NOT NULL REFERENCES companies(id), condo_id uuid NOT NULL, token_hash text UNIQUE NOT NULL, name text NOT NULL, active boolean NOT NULL DEFAULT true, last_seen timestamptz, FOREIGN KEY(company_id,condo_id) REFERENCES condos(company_id,id));
CREATE TABLE integrations (company_id uuid NOT NULL REFERENCES companies(id), kind text NOT NULL, status text NOT NULL DEFAULT 'disconnected', updated_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY(company_id,kind));
CREATE TABLE whatsapp_messages (company_id uuid NOT NULL REFERENCES companies(id), message_id text NOT NULL, sender text NOT NULL, response text, created_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY(company_id,message_id));
CREATE TABLE whatsapp_flows (company_id uuid NOT NULL REFERENCES companies(id), sender text NOT NULL, step text NOT NULL, data jsonb NOT NULL DEFAULT '{}', updated_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY(company_id,sender));
CREATE TABLE phone_links (id uuid PRIMARY KEY DEFAULT gen_random_uuid(), company_id uuid NOT NULL REFERENCES companies(id), subject text NOT NULL, code_hash text UNIQUE NOT NULL, expires_at timestamptz NOT NULL, used boolean NOT NULL DEFAULT false);
CREATE TABLE call_events (id uuid PRIMARY KEY DEFAULT gen_random_uuid(), company_id uuid NOT NULL REFERENCES companies(id), condo_id uuid NOT NULL, call_id text NOT NULL, kind text NOT NULL CHECK(kind IN ('ringing','answered','ended','missed')), created_at timestamptz NOT NULL DEFAULT now(), UNIQUE(company_id,call_id,kind), FOREIGN KEY(company_id,condo_id) REFERENCES condos(company_id,id));
CREATE INDEX records_scope ON records(company_id,condo_id,kind);
CREATE INDEX events_feed ON events(company_id,created_at DESC);
CREATE INDEX invitations_scope ON invitations(company_id,condo_id,unit_id);
CREATE INDEX commands_delivery ON commands(company_id,condo_id,state,expires_at);

ALTER TABLE platform_admins ENABLE ROW LEVEL SECURITY;
ALTER TABLE platform_admins FORCE ROW LEVEL SECURITY;
CREATE POLICY admin_self ON platform_admins FOR SELECT USING (subject=app.setting('subject'));
ALTER TABLE companies ENABLE ROW LEVEL SECURITY;
ALTER TABLE companies FORCE ROW LEVEL SECURITY;
CREATE POLICY companies_scope ON companies USING (app.is_platform() OR id=app.company()) WITH CHECK(app.is_platform());
ALTER TABLE memberships ENABLE ROW LEVEL SECURITY;
ALTER TABLE memberships FORCE ROW LEVEL SECURITY;
CREATE POLICY members_read ON memberships FOR SELECT USING(subject=app.setting('subject') OR (company_id=app.company() AND app.setting('role') IN ('company_admin','support')));
CREATE POLICY members_write ON memberships FOR ALL USING(company_id=app.company() AND app.setting('role') IN ('company_admin','support')) WITH CHECK(company_id=app.company() AND app.setting('role') IN ('company_admin','support'));
ALTER TABLE support_grants ENABLE ROW LEVEL SECURITY;
ALTER TABLE support_grants FORCE ROW LEVEL SECURITY;
CREATE POLICY support_scope ON support_grants USING(app.is_platform() AND subject=app.setting('subject')) WITH CHECK(app.is_platform() AND subject=app.setting('subject'));
ALTER TABLE domains ENABLE ROW LEVEL SECURITY;
ALTER TABLE domains FORCE ROW LEVEL SECURITY;
CREATE POLICY domains_scope ON domains USING(app.is_platform() OR company_id=app.company()) WITH CHECK(app.is_platform() OR (company_id=app.company() AND app.setting('role')='company_admin'));
ALTER TABLE condos ENABLE ROW LEVEL SECURITY;
ALTER TABLE condos FORCE ROW LEVEL SECURITY;
CREATE POLICY condos_scope ON condos USING(company_id=app.company() AND app.can_condo(id)) WITH CHECK(company_id=app.company() AND app.setting('role') IN ('company_admin','support'));

DO $$ DECLARE t text; BEGIN
 FOREACH t IN ARRAY ARRAY['records','invitations','commands','events','tickets','agent_keys','call_events'] LOOP
  EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY',t);
  EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY',t);
  EXECUTE format('CREATE POLICY tenant_scope ON %I USING(company_id=app.company() AND (condo_id IS NULL OR app.can_condo(condo_id))) WITH CHECK(company_id=app.company() AND (condo_id IS NULL OR app.can_condo(condo_id)))',t);
 END LOOP;
 FOREACH t IN ARRAY ARRAY['integrations','whatsapp_messages','whatsapp_flows','phone_links'] LOOP
  EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY',t);
  EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY',t);
  EXECUTE format('CREATE POLICY tenant_scope ON %I USING(company_id=app.company()) WITH CHECK(company_id=app.company())',t);
 END LOOP;
END $$;
CREATE POLICY resident_records ON records AS RESTRICTIVE USING(app.setting('role')<>'resident' OR (kind='units' AND app.can_unit(id)) OR (kind IN ('people','vehicles','credentials') AND app.can_unit(unit_id)) OR kind IN ('access-points','rules'));
CREATE POLICY resident_invites ON invitations AS RESTRICTIVE USING(app.can_unit(unit_id));
CREATE POLICY resident_events ON events AS RESTRICTIVE USING(app.can_unit(unit_id));
CREATE POLICY resident_tickets ON tickets AS RESTRICTIVE USING(app.can_unit(unit_id));
CREATE POLICY deny_resident_commands ON commands AS RESTRICTIVE USING(app.setting('role')<>'resident');
CREATE POLICY deny_resident_agents ON agent_keys AS RESTRICTIVE USING(app.setting('role') IN ('company_admin','support','agent'));
CREATE POLICY deny_resident_calls ON call_events AS RESTRICTIVE USING(app.setting('role')<>'resident');

-- Narrow capability lookups: only resolve a hashed bearer token, never expose whole rows.
CREATE FUNCTION app.resolve_invite(hash text) RETURNS TABLE(company_id uuid,condo_id uuid,id uuid) LANGUAGE sql SECURITY DEFINER SET search_path=public,pg_temp AS $$ SELECT i.company_id,i.condo_id,i.id FROM invitations i JOIN companies c ON c.id=i.company_id WHERE i.token_hash=hash AND c.status='active' $$;
CREATE FUNCTION app.resolve_agent(hash text) RETURNS TABLE(company_id uuid,condo_id uuid,id uuid) LANGUAGE sql SECURITY DEFINER SET search_path=public,pg_temp AS $$ SELECT a.company_id,a.condo_id,a.id FROM agent_keys a JOIN companies c ON c.id=a.company_id WHERE a.token_hash=hash AND a.active AND c.status='active' $$;
CREATE FUNCTION app.resolve_host(host text) RETURNS TABLE(company_id uuid,brand jsonb) LANGUAGE sql SECURITY DEFINER SET search_path=public,pg_temp AS $$ SELECT c.id,c.brand FROM domains d JOIN companies c ON c.id=d.company_id WHERE d.hostname=host AND d.verified_at IS NOT NULL AND d.tls_checked_at IS NOT NULL AND c.status='active' $$;
CREATE FUNCTION app.whatsapp_member(phone text) RETURNS SETOF memberships LANGUAGE sql SECURITY DEFINER SET search_path=public,pg_temp AS $$ SELECT m.* FROM memberships m WHERE m.company_id=app.company() AND m.whatsapp=phone AND m.whatsapp_verified AND m.active AND m.role='resident' $$;
REVOKE ALL ON ALL FUNCTIONS IN SCHEMA app FROM PUBLIC;
GRANT USAGE ON SCHEMA app TO portaria_app;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA app TO portaria_app;
GRANT SELECT,INSERT,UPDATE,DELETE ON ALL TABLES IN SCHEMA public TO portaria_app;
REVOKE UPDATE,DELETE ON events FROM portaria_app;
REVOKE INSERT,UPDATE,DELETE ON platform_admins FROM portaria_app;
CREATE FUNCTION app.agent_online(company uuid,condo uuid) RETURNS boolean LANGUAGE sql SECURITY DEFINER SET search_path=public,pg_temp AS $$ SELECT company=app.company() AND app.can_condo(condo) AND EXISTS(SELECT 1 FROM agent_keys WHERE company_id=company AND condo_id=condo AND active AND last_seen>now()-interval '60 seconds') $$;
CREATE FUNCTION app.device_ready(company uuid,device uuid) RETURNS boolean LANGUAGE sql SECURITY DEFINER SET search_path=public,pg_temp AS $$ SELECT EXISTS(SELECT 1 FROM records WHERE company_id=company AND company=app.company() AND app.can_condo(condo_id) AND id=device AND kind='devices' AND data->>'homologated'='true') $$;
REVOKE ALL ON FUNCTION app.agent_online(uuid,uuid),app.device_ready(uuid,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION app.agent_online(uuid,uuid),app.device_ready(uuid,uuid) TO portaria_app;
CREATE FUNCTION app.company_label(company uuid) RETURNS text LANGUAGE sql SECURITY DEFINER SET search_path=public,pg_temp AS $$ SELECT c.name FROM companies c WHERE c.id=company AND EXISTS(SELECT 1 FROM memberships m WHERE m.company_id=c.id AND m.subject=app.setting('subject') AND m.active) $$;
REVOKE ALL ON FUNCTION app.company_label(uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION app.company_label(uuid) TO portaria_app;
