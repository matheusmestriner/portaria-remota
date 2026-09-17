# Implantação e continuidade

## Ambientes

Mantenha desenvolvimento, homologação e produção em redes, bancos, realms Keycloak e segredos separados. Não copie dados reais para testes. Execute migrações por conta privilegiada de implantação; API e worker usam `portaria_runtime`, sem superusuário nem bypass de RLS.

## Publicação

1. Contrate infraestrutura e configure DNS para painel, identidade, API e mídia. Defina `PUBLIC_WEB_URL`, `OIDC_ISSUER`, `DOMAIN_TARGET`, `MEDIA_PUBLIC_URL`, `SIP_WEBSOCKET_URL` e `SIP_DOMAIN`.
2. Use PostgreSQL persistente, com backups e armazenamento protegido. Keycloak precisa de banco externo em produção, `start`, hostname HTTPS fixo, proxy headers restritos e credenciais de bootstrap removidas/rotacionadas após implantação.
3. Mantenha o fluxo de autenticação fornecido exigindo senha e OTP, sem direct grants, service accounts nos clientes públicos ou provedores alternativos não avaliados. O claim `acr=2` foi configurado para esse fluxo integralmente obrigatório. Não flexibilize o fluxo sem substituir essa configuração por LoA nativo e revalidar a verificação da API.
4. A recuperação de conta deve preservar a exigência de segundo fator. Revise reset credentials, ações administrativas e emissão de tokens no teste completo de Keycloak antes da produção.
5. Registre URLs reais de redirecionamento no cliente `portaria-web` e `portaria://auth` no cliente móvel. Não use `*` como origem global. Cada domínio white label é autorizado separadamente após a verificação.
6. Use o Caddyfile como base de proxy TLS. Proteja a chave usada pelo endpoint de autorização de certificados e não registre essa query em logs. Endpoints `/v1/internal/*` não devem ficar públicos.
7. API, bancos, Redis, MediaMTX administrativo, Asterisk administrativo e bridges ficam em rede privada. Agentes de condomínios usam saída HTTPS. Habilite `TRUST_PROXY=1` somente quando a API for alcançável exclusivamente por um proxy confiável.
8. Configure métricas externas para API, worker, PostgreSQL, Redis, armazenamento, certificado, telefonia e perda de conectividade. O worker abre ocorrências para agentes desconectados e fecha essas ocorrências após recuperação.
9. Homologue cada modelo/firmware de equipamento. Realize o piloto com clientes cadastrados por você. Teste contingência local, queda de internet e recuperação antes de disponibilizar a operação comercial.

O projeto não contrata servidor, não altera DNS externo, não emite certificados sem seu domínio, não conecta um número WhatsApp e não publica aplicativos automaticamente.

## Backups

Use `scripts/backup.ps1` com as ferramentas cliente PostgreSQL e a variável `MIGRATION_DATABASE_URL`. O script gera um dump e seu SHA-256. Mantenha cópia cifrada fora do servidor da aplicação e teste uma política de retenção apropriada ao contrato de cada cliente.

Faça também backup separado do Keycloak, dos bancos de sessão Whatsmeow e dos volumes/configurações locais necessários. **Guarde `DATA_KEY` em um cofre separado**: sem ela, o conteúdo cifrado não pode ser recuperado. Backups e relatórios não devem conter segredos em arquivos versionados.

`scripts/restore-check.ps1` só aceita um banco vazio cujo nome comece por `restore_test_`. Crie-o em um ambiente de teste que já tenha os papéis PostgreSQL necessários, restaure o dump e confira as contagens. Depois execute testes autenticados, RLS, leitura dos dados cifrados, login e conexão dos serviços. Não use esse procedimento para substituir produção diretamente.

Ainda não foi realizado um ensaio real de restauração: não havia PostgreSQL/Docker nesta máquina. Um teste de dump não substitui a validação de recuperação completa e a medição de perda de dados/tempo de retomada.

## Limites atuais e liberação comercial

- A API/PWA foram compiladas; o app Expo e os serviços Go precisam de build e testes próprios em ambiente compatível.
- Os testes de banco usam PostgreSQL/WASM em memória. Repita-os contra a versão PostgreSQL do servidor, incluindo concorrência, conexões agrupadas e carga real.
- A inspeção visual pelo navegador foi recusada nesta sessão; não há captura de tela nem validação visual certificada.
- As dependências Node estão sem vulnerabilidades na validação atual (`npm audit --audit-level=high`). Repita essa verificação em cada atualização e revise incompatibilidades antes de usar atualizações forçadas.
- As dependências Go da ponte Whatsmeow ainda precisam ser resolvidas e fixadas em `go.mod`/`go.sum`. O Dockerfile fixa a revisão upstream consultada; seu build não foi executado.
- Não há um adaptador nativo de fabricante homologado, fila durável de eventos físicos no agente, motor de biometria, gravação de vídeo ou dialplan homologado para um PABX específico. A ponte AMI foi fornecida em código e precisa de configuração e teste real.
- Múltiplas instâncias exigem revisão de rate limiting distribuído, registro de telefonia por operador e propriedade das sessões WhatsApp. Não publique várias réplicas do mesmo bridge contra o mesmo número.
- Eventos de auditoria não são apagados pela aplicação. Defina política de retenção e arquivamento com cada cliente antes de começar a coletar dados reais.
- Identidade da marca e identificadores de loja são provisórios. Ajuste-os antes da distribuição pública.

## Critério de aprovação

O sistema pode ser liberado para clientes somente após: autenticação/MFA real, isolamento de tenants nos endpoints e mídia, homologação do hardware, chamada real com áudio bidirecional, pareamento WhatsApp, builds Android/iOS, verificação de acessibilidade/responsividade, auditoria de dependências corrigida e recuperação completa de backup.

