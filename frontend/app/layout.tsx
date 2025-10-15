import './globals.css';
import type { Metadata } from 'next';
import { ReactNode } from 'react';

export const metadata: Metadata = {
  title: 'Yamato Dashboard',
  description: 'Operational view backed by the Yamato Go API',
};

interface RootLayoutProps {
  children: ReactNode;
}

//1.- RootLayout renders the global document shell for the Next.js frontend.
export default function RootLayout({ children }: RootLayoutProps) {
  return (
    <html lang="en" className="bg-slate-100">
      <body className="min-h-screen font-sans text-slate-800">
        {/*2.- Provide a centered container to host all route-specific content.*/}
        <main className="mx-auto max-w-5xl px-6 py-10">{children}</main>
      </body>
    </html>
  );
}
