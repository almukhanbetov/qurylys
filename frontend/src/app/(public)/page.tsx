import Link from "next/link";
import { getI18n } from "@/lib/i18n/server";
import { format, pick } from "@/lib/i18n";
import { api } from "@/lib/api";
import { SearchForm } from "@/components/layout/SearchForm";
import { Icon, categoryIcon } from "@/components/ui/Icon";
import { ButtonLink, UnavailableButton } from "@/components/ui/Button";
import { SectionError } from "@/components/ui/RetryButton";
import { EmptyState } from "@/components/ui/StateMessage";
import { ProductCard, ProductGrid } from "@/features/products/ProductCard";

export const dynamic = "force-dynamic";

export default async function HomePage() {
  const { t, locale } = await getI18n();

  // Independent requests: one failing section does not take the page down.
  const [treeRes, productsRes] = await Promise.allSettled([
    api.categoryTree(),
    api.products({ sort: "offers", pageSize: 8 }),
  ]);

  return (
    <>
      {/* Hero */}
      <section className="bg-gradient-to-b from-brand-50 to-ink-50">
        <div className="mx-auto grid max-w-7xl items-center gap-10 px-4 py-12 sm:px-6 md:py-20 lg:grid-cols-[1.2fr_1fr]">
          <div>
            <h1 className="text-3xl font-extrabold leading-tight tracking-tight sm:text-4xl lg:text-5xl">
              {t.home.heroTitle}
            </h1>
            <p className="mt-4 max-w-xl text-base text-ink-600 sm:text-lg">{t.home.heroText}</p>
            <div className="mt-8 max-w-xl">
              <SearchForm
                size="lg"
                label={t.home.heroSearchLabel}
                placeholder={t.header.searchPlaceholder}
                buttonLabel={t.header.searchButton}
              />
            </div>
            <div className="mt-4">
              <ButtonLink href="/catalog" variant="ghost">
                {t.home.heroCta}
                <Icon name="chevronRight" className="h-4 w-4" />
              </ButtonLink>
            </div>
          </div>
          <div aria-hidden="true" className="hidden lg:block">
            <div className="grid grid-cols-2 gap-4">
              {(["bricks", "layers", "beam", "roof"] as const).map((name, i) => (
                <div
                  key={name}
                  className={`grid aspect-square place-items-center rounded-3xl ${
                    i % 3 === 0 ? "bg-brand-700 text-brand-100" : "bg-white text-brand-600 ring-1 ring-brand-100"
                  }`}
                >
                  <Icon name={name} className="h-16 w-16" />
                </div>
              ))}
            </div>
          </div>
        </div>
      </section>

      {/* Categories */}
      <section className="mx-auto max-w-7xl px-4 py-12 sm:px-6" aria-labelledby="home-categories">
        <div className="flex items-end justify-between gap-4">
          <h2 id="home-categories" className="text-2xl font-bold">{t.home.categoriesTitle}</h2>
          <Link href="/categories" className="text-sm font-semibold text-brand-700 hover:underline">
            {t.home.allCategories}
          </Link>
        </div>
        <div className="mt-6">
          {treeRes.status === "rejected" ? (
            <SectionError title={t.common.errorTitle} text={t.common.errorText} />
          ) : treeRes.value.length === 0 ? (
            <EmptyState title={t.categories.empty} />
          ) : (
            <ul className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-6">
              {treeRes.value.map((c) => (
                <li key={c.id}>
                  <Link
                    href={`/catalog?category=${c.slug}`}
                    className="flex h-full flex-col gap-3 rounded-2xl bg-white p-4 ring-1 ring-ink-100 transition hover:ring-brand-300 hover:shadow-md hover:shadow-brand-900/5"
                  >
                    <span className="grid h-11 w-11 place-items-center rounded-xl bg-brand-50 text-brand-700">
                      <Icon name={categoryIcon(c.slug)} className="h-6 w-6" />
                    </span>
                    <span className="text-sm font-semibold text-ink-900">{pick(locale, c.name_kk, c.name_ru)}</span>
                    <span className="mt-auto text-xs text-ink-500">
                      {format(t.home.productsCount, { count: c.product_count })}
                    </span>
                  </Link>
                </li>
              ))}
            </ul>
          )}
        </div>
      </section>

      {/* Products with most offers */}
      <section className="mx-auto max-w-7xl px-4 py-4 sm:px-6" aria-labelledby="home-products">
        <div className="flex flex-col gap-1 sm:flex-row sm:items-end sm:justify-between">
          <div>
            <h2 id="home-products" className="text-2xl font-bold">{t.home.popularTitle}</h2>
            <p className="mt-1 text-sm text-ink-500">{t.home.popularText}</p>
          </div>
          <Link href="/catalog?sort=offers" className="text-sm font-semibold text-brand-700 hover:underline">
            {t.home.showAll}
          </Link>
        </div>
        <p className="mt-4 rounded-xl bg-amber-50 px-4 py-2.5 text-xs text-amber-900 ring-1 ring-amber-100">
          {t.common.demoNotice}
        </p>
        <div className="mt-6">
          {productsRes.status === "rejected" ? (
            <SectionError title={t.common.errorTitle} text={t.common.errorText} />
          ) : productsRes.value.items.length === 0 ? (
            <EmptyState title={t.catalog.emptyTitle} />
          ) : (
            <ProductGrid>
              {productsRes.value.items.map((p) => (
                <ProductCard key={p.id} product={p} locale={locale} t={t} />
              ))}
            </ProductGrid>
          )}
        </div>
      </section>

      {/* For suppliers */}
      <section className="mx-auto max-w-7xl px-4 py-12 sm:px-6" aria-labelledby="home-suppliers">
        <div className="grid gap-8 rounded-3xl bg-brand-800 p-8 text-white sm:p-10 lg:grid-cols-[1.3fr_1fr] lg:items-center">
          <div>
            <h2 id="home-suppliers" className="text-2xl font-bold text-white sm:text-3xl">{t.home.suppliersTitle}</h2>
            <p className="mt-3 max-w-2xl text-brand-100">{t.home.suppliersText}</p>
            <div className="mt-6">
              <UnavailableButton soonLabel={t.common.soon} title={t.common.stageNotice}>
                {t.home.suppliersCta}
              </UnavailableButton>
            </div>
          </div>
          <ul className="space-y-3">
            {[t.home.suppliersPoint1, t.home.suppliersPoint2, t.home.suppliersPoint3].map((point) => (
              <li key={point} className="flex items-start gap-3 rounded-2xl bg-brand-700/60 p-4">
                <span className="grid h-7 w-7 shrink-0 place-items-center rounded-full bg-brand-100 text-brand-800">
                  <Icon name="check" className="h-4 w-4" />
                </span>
                <span className="text-sm text-brand-50">{point}</span>
              </li>
            ))}
          </ul>
        </div>
      </section>
    </>
  );
}
