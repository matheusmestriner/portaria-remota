# Deploy no Render

O arquivo `render.yaml` descreve a API, o painel web, o worker, PostgreSQL e Redis. Antes de aplicar o Blueprint, configure no painel os valores marcados como `sync: false`.

Os valores mínimos são:

- `DATA_KEY`: 64 caracteres hexadecimais, gerados com `openssl rand -hex 32`;
- `DATABASE_URL`: conexão de runtime com usuário PostgreSQL sem superusuário e sem `BYPASSRLS`;
- `OIDC_ISSUER` e `OIDC_JWKS_URL`: URLs do provedor Keycloak/OIDC publicado;
- `DOMAIN_TARGET`: hostname de entrada que será usado no CNAME;
- `OIDC_ISSUER` no painel web, igual ao valor usado pela API.

O banco deve ser migrado e receber o bootstrap antes da primeira operação. Não cadastre dados fictícios para validar o ambiente de produção. Use o script de homologação apenas em um banco separado.

O PABX Asterisk, o Whatsmeow e os agentes locais permanecem serviços separados. Eles não devem ser expostos diretamente pelo Blueprint web; conecte-os por rede privada, túnel ou infraestrutura própria após a homologação.
