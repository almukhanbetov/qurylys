"use client";

import type { ReactNode } from "react";

/** Search form for public lists; the URL keeps the query, so results can be shared. */
export function SearchRow({
  label,
  button,
  initial,
  onSearch,
  children,
}: {
  label: string;
  button: string;
  initial: string;
  onSearch: (q: string) => void;
  children?: ReactNode;
}) {
  return (
    <form
      role="search"
      className="flex flex-col gap-3 sm:flex-row sm:items-end"
      onSubmit={(e) => {
        e.preventDefault();
        const q = new FormData(e.currentTarget).get("q");
        onSearch(typeof q === "string" ? q.trim() : "");
      }}
    >
      <label className="block min-w-0 flex-1 text-sm font-medium text-ink-900">
        {label}
        <input
          key={initial}
          name="q"
          type="search"
          defaultValue={initial}
          maxLength={100}
          className="mt-1 h-11 w-full rounded-xl border border-ink-300 bg-white px-3 text-base text-ink-900 focus:border-brand-700 focus:outline-none focus:ring-2 focus:ring-brand-200"
        />
      </label>
      {children}
      <button
        type="submit"
        className="h-11 cursor-pointer rounded-xl bg-brand-700 px-5 text-sm font-semibold text-white hover:bg-brand-800 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-brand-600"
      >
        {button}
      </button>
    </form>
  );
}
