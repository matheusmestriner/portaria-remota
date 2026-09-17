'use client';
import ErrorPage from './ui/error-page';
export default function GlobalError(){return <html lang="pt-BR"><body><ErrorPage code="500" title="A plataforma precisa de um instante" message="Não foi possível concluir o carregamento. Tente novamente em alguns instantes." retry/></body></html>}

