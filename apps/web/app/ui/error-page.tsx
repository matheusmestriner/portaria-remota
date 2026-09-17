'use client';
import Link from 'next/link';
import { ArrowLeft, Home, RefreshCw, ShieldAlert } from 'lucide-react';
type Props={code:'403'|'404'|'429'|'500';title:string;message:string;retry?:boolean};
export default function ErrorPage({code,title,message,retry=false}:Props){return <main className="error-page"><div className="error-mark"><ShieldAlert size={28}/></div><span className="error-code">ERRO {code}</span><h1>{title}</h1><p>{message}</p><div className="error-actions">{retry&&<button className="btn primary" onClick={()=>location.reload()}><RefreshCw size={16}/> Tentar novamente</button>}<Link className="btn secondary" href="/"><Home size={16}/> Ir para o início</Link><button className="error-back" onClick={()=>history.length>1?history.back():location.assign('/')}><ArrowLeft size={15}/> Voltar</button></div></main>}

