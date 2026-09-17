'use client';
import ErrorPage from './ui/error-page';
export default function Error(){return <ErrorPage code="500" title="Não foi possível carregar esta página" message="Ocorreu um erro inesperado. Tente novamente em alguns instantes." retry/>}

