import React from 'react';
import Link from '@docusaurus/Link';

interface DocCardProps {
  title: string;
  description: string;
  href: string;
  icon?: string;
  badge?: string;
}

export default function DocCard({ title, description, href, icon, badge }: DocCardProps): React.JSX.Element {
  return (
    <Link
      to={href}
      className="group no-underline relative block p-6 bg-zinc-900/60 dark:bg-zinc-900/60 border border-zinc-800 dark:border-zinc-800 rounded-xl overflow-hidden hover:border-zinc-400 dark:hover:border-zinc-300 hover:shadow-lg transition-all duration-300 transform hover:-translate-y-1 text-zinc-100 dark:text-zinc-200"
    >
      {/* Decorative hover glow backdrop */}
      <div className="absolute inset-0 bg-gradient-to-br from-zinc-200/0 via-zinc-200/0 to-zinc-200/5 dark:to-zinc-200/5 opacity-0 group-hover:opacity-100 transition-opacity duration-300 pointer-events-none" />
      
      <div className="flex items-start justify-between mb-4">
        {icon ? (
          <span className="text-2xl p-2.5 bg-zinc-800/80 dark:bg-zinc-800/80 text-zinc-300 dark:text-zinc-200 rounded-lg group-hover:scale-110 transition-transform duration-300">
            {icon}
          </span>
        ) : (
          <span className="text-2xl p-2.5 bg-zinc-800/80 dark:bg-zinc-800/80 text-zinc-300 dark:text-zinc-200 rounded-lg group-hover:scale-110 transition-transform duration-300">
            📖
          </span>
        )}
        
        {badge && (
          <span className="text-[10px] uppercase font-bold tracking-wider px-2 py-0.5 bg-zinc-850 dark:bg-zinc-850 text-zinc-400 dark:text-zinc-300 rounded-full border border-zinc-800 dark:border-zinc-700">
            {badge}
          </span>
        )}
      </div>

      <h3 className="text-lg font-semibold font-heading mb-2 group-hover:text-white dark:group-hover:text-white transition-colors">
        {title}
      </h3>
      
      <p className="text-sm text-zinc-400 dark:text-zinc-400 leading-relaxed font-sans m-0">
        {description}
      </p>

      {/* Modern link arrow indicator */}
      <div className="mt-4 flex items-center text-xs font-semibold text-zinc-400 dark:text-zinc-300 group-hover:underline">
        Read documentation
        <svg
          className="w-3 h-3 ml-1.5 transform group-hover:translate-x-1 transition-transform"
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2.5}
            d="M9 5l7 7-7 7"
          />
        </svg>
      </div>
    </Link>
  );
}
