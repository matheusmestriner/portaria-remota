import { randomBytes } from 'node:crypto';
import { writeFile,mkdir } from 'node:fs/promises';
const secret=()=>randomBytes(32).toString('hex');const root=secret(),app=secret();
const data=`# Gerado localmente; nunca versionar.\nPOSTGRES_PASSWORD=${root}\nDB_APP_PASSWORD=${app}\nMIGRATION_DATABASE_URL=postgres://postgres:${root}@localhost:5432/portaria\nDATABASE_URL=postgres://portaria_runtime:${app}@localhost:5432/portaria\nDATA_KEY=${secret()}\nINTERNAL_KEY=${secret()}\nWHATSAPP_KEY=${secret()}\nSETUP_TOKEN=${secret()}\nKC_BOOTSTRAP_ADMIN_USERNAME=bootstrap\nKC_BOOTSTRAP_ADMIN_PASSWORD=${secret()}\nOIDC_ISSUER=http://localhost:8080/realms/portaria\nPUBLIC_WEB_URL=http://localhost:3000\nWEB_ORIGINS=http://localhost:3000,http://127.0.0.1:3000\nREDIS_URL=redis://localhost:6379\nDOMAIN_TARGET=\nAPI_PORT=4000\nAPI_HOST=127.0.0.1\n`;
await writeFile('.env',data,{flag:'wx',mode:0o600});console.log('.env criado com segredos exclusivos. Nenhum cliente ou usuário de demonstração foi criado.');

