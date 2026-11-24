import { Icon } from "@/components/ui/Icon";

/**
 * Plain GET form to /catalog?q=…, so search works even before JavaScript loads.
 * White field for the dark-green header; the button shrinks to an icon on phones.
 */
export function SearchForm({ placeholder, buttonLabel }: { placeholder: string; buttonLabel: string }) {
  return (
    <form
      action="/catalog"
      method="get"
      role="search"
      className="flex h-11 w-full items-stretch rounded-xl bg-white p-1 focus-within:ring-2 focus-within:ring-accent-400"
    >
      <label className="min-w-0 flex-1">
        <span className="sr-only">{placeholder}</span>
        {/* 16px on phones: iOS zooms the page into smaller inputs. */}
        <input
          type="search"
          name="q"
          maxLength={100}
          placeholder={placeholder}
          className="h-full w-full text-ellipsis bg-transparent px-3 text-base text-ink-900 placeholder:text-ink-500 focus:outline-none md:text-sm"
        />
      </label>
      <button
        type="submit"
        className="flex shrink-0 cursor-pointer items-center gap-2 rounded-lg bg-brand-700 px-3 text-sm font-semibold text-white hover:bg-brand-800 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-brand-600 md:px-4"
      >
        <Icon name="search" className="h-5 w-5 md:h-4 md:w-4" />
        <span className="sr-only md:not-sr-only">{buttonLabel}</span>
      </button>
    </form>
  );
}
