import { Icon } from "@/components/ui/Icon";

/**
 * Plain GET form to /catalog?q=…, so search works even before JavaScript loads.
 */
export function SearchForm({
  placeholder,
  buttonLabel,
  defaultValue,
  size = "md",
  label,
}: {
  placeholder: string;
  buttonLabel: string;
  defaultValue?: string;
  size?: "md" | "lg";
  label?: string;
}) {
  const lg = size === "lg";
  return (
    <form action="/catalog" method="get" role="search" className="flex w-full items-stretch gap-2">
      <label className="relative flex-1">
        <span className="sr-only">{label ?? placeholder}</span>
        <Icon
          name="search"
          className={`pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-ink-400 ${lg ? "h-5 w-5" : "h-4 w-4"}`}
        />
        <input
          type="search"
          name="q"
          defaultValue={defaultValue}
          maxLength={100}
          placeholder={placeholder}
          className={`w-full rounded-xl border border-ink-200 bg-white pl-10 pr-3 text-ink-900 placeholder:text-ink-400 focus:border-brand-500 focus:outline-none focus:ring-2 focus:ring-brand-200 ${
            lg ? "h-14 text-base" : "h-11 text-sm"
          }`}
        />
      </label>
      <button
        type="submit"
        className={`shrink-0 rounded-xl bg-brand-700 font-semibold text-white hover:bg-brand-800 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-brand-600 ${
          lg ? "h-14 px-6 text-base" : "h-11 px-4 text-sm"
        }`}
      >
        {buttonLabel}
      </button>
    </form>
  );
}
