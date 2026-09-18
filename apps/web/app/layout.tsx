import type { Metadata } from 'next';
import './globals.css';
import './monochrome.css';
export const metadata:Metadata={title:'Vértice · Portaria remota',description:'Sua operação de acesso, em um só lugar.',manifest:'/manifest.webmanifest',icons:{icon:'/icon.svg',apple:'/icon-192.png'},robots:{index:false,follow:false},appleWebApp:{capable:true,title:'Vértice',statusBarStyle:'default'}};
export default function Layout({children}:{children:React.ReactNode}){return <html lang="pt-BR"><body>{children}</body></html>;}