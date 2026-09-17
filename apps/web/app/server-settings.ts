import { loadEnvFile } from 'node:process';
import { resolve } from 'node:path';
try{loadEnvFile(resolve(process.cwd(),'../../.env'));}catch{}
export const settings={api:process.env.API_URL||'http://127.0.0.1:4000',issuer:process.env.OIDC_ISSUER,domainTarget:process.env.DOMAIN_TARGET};
