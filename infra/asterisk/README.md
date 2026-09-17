# Base do PABX Asterisk

Esta pasta contém a base do PABX usado pela central de portaria. Ela inicia sem ramais, empresas, filas de atendimento ou senhas fictícias. Os arquivos `*.example` são modelos comentados para gerar os arquivos reais durante o provisionamento de cada cliente.

## Subir localmente

1. Copie os exemplos para arquivos de configuração reais, preenchendo apenas dados de clientes homologados:

   - `pjsip-clients.conf`
   - `extensions-clients.conf`
   - `manager-clients.conf`

2. Inicie o serviço:

   ```bash
   docker compose -f docker-compose.pbx.yaml up --build
   ```

3. Verifique o processo no contêiner:

   ```bash
   docker compose -f docker-compose.pbx.yaml exec asterisk asterisk -rx "core show uptime"
   docker compose -f docker-compose.pbx.yaml exec asterisk asterisk -rx "pjsip show endpoints"
   ```

O compose publica SIP e RTP para os equipamentos homologados e mantém a interface HTTP/AMI presa a `127.0.0.1`. Em produção, o AMI deve continuar em rede privada; o serviço `services/telephony` é o consumidor dos eventos `UserEvent(PortariaCall, ...)`.

## Provisionamento real

O serviço de provisionamento deve gerar os includes a partir do banco, sempre com:

- um contexto de discagem por empresa e condomínio;
- filas contendo somente operadores atribuídos àquele atendimento;
- identificadores UUID para empresa, condomínio, acesso e chamada;
- segredos fora do Git, montados como secret do ambiente;
- remoção ou revogação de ramais desativados;
- reload validado pelo Asterisk antes de marcar a configuração como ativa.

Não use um contexto genérico que aceite qualquer destino. A base rejeita destinos não provisionados e não possui rota de saída para troncos externos.

## WebRTC e segurança

`http.conf` começa com TLS desligado para facilitar bancada local. Antes de usar navegador ou aplicativo em ambiente público:

1. monte certificado e chave privados como secrets;
2. habilite TLS no `http.conf`;
3. configure o transporte `wss` do `pjsip.conf` com os mesmos certificados;
4. restrinja firewall, origem WebSocket e portas RTP;
5. valide chamadas em Android, iPhone e navegador.

O arquivo `pjsip.conf` usa certificados DTLS para WebRTC, mas nenhum certificado é criado automaticamente. Isso evita que uma instalação vazia pareça pronta para produção.

## Eventos de chamada

Os dialplans de cliente devem chamar `Gosub(portaria-notify,s,1(...))` para `ringing`, `answered`, `ended` e `missed`. O bridge AMI converte esses eventos para a API, que grava atendimento, operador, acesso e resultado do comando. Uma chamada atendida não equivale a uma passagem pelo portão; a passagem depende do evento do agente/equipamento.

## Limites desta base

Esta imagem não escolhe fabricante de controladora, câmera ou interfone e não promete compatibilidade universal. A homologação deve ser feita com o conjunto real de equipamentos de cada cliente. A gravação contínua de áudio/vídeo também permanece fora desta primeira base.
