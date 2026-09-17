# Estrutura adotada a partir do PortalIA

O projeto mantém a ideia central do `portaIA`: painel web, API, banco multiempresa, serviços separados para WhatsApp e uma operação preparada para mídia e telefonia. A organização foi adaptada para a operação de portaria remota e para publicação segura.

| PortalIA | Plataforma atual | Melhoria aplicada |
| --- | --- | --- |
| `frontend` | `apps/web` | Next.js com PWA, white label, convite público e páginas de erro próprias |
| `backend` | `apps/api` | NestJS modular, contratos versionados, validação Zod e autorização no servidor |
| `services/whatsapp` | `services/whatsmeow` | Serviço Go desacoplado, mensagens idempotentes e somente convite antecipado |
| PostgreSQL por `tenant_id` | PostgreSQL com empresa + condomínio | RLS forçado, chaves compostas e escopo aplicado também em tarefas em segundo plano |
| Redis preparado | Redis + BullMQ | Filas para processamento assíncrono e integração pronta para limite distribuído |
| MinIO/S3 preparado | Mídia privada com tokens curtos | Câmeras autorizadas por empresa, condomínio e caminho da transmissão |
| Seeds de demonstração | Instalação vazia | Nenhum cliente, morador ou movimentação fictícia em produção |

## Diferenças de segurança

- Não há credencial padrão no README ou no ambiente de produção.
- O Swagger fica desativado em produção.
- O agente local usa token com expiração e rotação.
- Convites e comandos têm idempotência e prazo curto.
- A API impede conexão PostgreSQL com superusuário ou `BYPASSRLS`.
- Suporte da plataforma a um cliente exige motivo, registro de auditoria e expira em 30 minutos.
- O painel não expõe diretamente os serviços internos, equipamentos ou segredos de integração.

O próximo passo de infraestrutura é substituir os contadores de requisição em memória por Redis quando houver mais de uma réplica da API.

