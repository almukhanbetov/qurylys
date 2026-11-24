import Image from "next/image";
import Link from "next/link";
import { getI18n } from "@/lib/i18n/server";
import { format, pick } from "@/lib/i18n";
import { api } from "@/lib/api";
// Photo: "Bricks for construction" by Saral Shots, CC0, cropped.
// https://commons.wikimedia.org/wiki/File:Bricks_for_construction.jpg
import heroPhoto from "@/assets/hero-bricks.webp";
import { Icon, categoryIcon } from "@/components/ui/Icon";
import { ButtonLink } from "@/components/ui/Button";
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
      {/* Hero: text on dark green, photo on the right (below the text on phones) */}
      <section className="mx-auto max-w-7xl px-4 pt-4 sm:px-6 sm:pt-6" aria-labelledby="home-hero">
        <div className="grid overflow-hidden rounded-2xl bg-brand-900 md:grid-cols-2">
          <div className="p-6 sm:p-8 lg:p-12">
            <h1
              id="home-hero"
              className="text-2xl font-extrabold leading-tight tracking-tight text-white sm:text-3xl lg:text-4xl"
            >
              {t.home.heroTitle}
            </h1>
            <span aria-hidden="true" className="mt-4 block h-1 w-20 rounded-full bg-accent-400" />
            <p className="mt-4 max-w-xl text-sm text-brand-100 sm:text-base">{t.home.heroText}</p>
            <ButtonLink href="/catalog" className="mt-6 ring-1 ring-white/30">
              {t.home.heroCta}
              <Icon name="chevronRight" className="h-4 w-4" />
            </ButtonLink>
          </div>
          <div className="relative min-h-40 md:min-h-72">
            <div
              aria-hidden="true"
              className="absolute inset-0 bg-accent-400 md:[clip-path:polygon(18%_0,100%_0,100%_100%,0_100%)]"
            />
            {/* Already optimized WebP, so it is served as is with immutable caching. */}
            <Image
              src={heroPhoto}
              alt=""
              preload
              unoptimized
              className="absolute inset-x-0 bottom-0 top-1 h-[calc(100%-0.25rem)] w-full object-cover md:top-0 md:h-full md:[clip-path:polygon(21%_0,100%_0,100%_100%,3%_100%)]"
            />
          </div>
        </div>
      </section>

      {/* Categories */}
      <section className="mx-auto max-w-7xl px-4 py-8 sm:px-6 sm:py-10" aria-labelledby="home-categories">
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
                    className="group flex h-full flex-col items-center gap-2 rounded-2xl border border-ink-200 bg-white p-4 text-center transition-colors hover:border-brand-700 hover:bg-brand-100 focus-visible:border-brand-700 focus-visible:bg-brand-100 focus-visible:outline-2 focus-visible:outline-brand-700"
                  >
                    <span className="grid h-12 w-12 place-items-center text-brand-700">
                      <Icon name={categoryIcon(c.slug)} className="h-9 w-9" />
                    </span>
                    <span className="w-full text-sm wrap-break-word font-semibold text-ink-900">{pick(locale, c.name_kk, c.name_ru)}</span>
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
        <div className="grid gap-8 rounded-2xl bg-brand-900 p-6 text-white sm:p-10 lg:grid-cols-[1.3fr_1fr] lg:items-center">
          <div>
            <h2 id="home-suppliers" className="text-2xl font-bold text-white sm:text-3xl">{t.home.suppliersTitle}</h2>
            <p className="mt-3 max-w-2xl text-brand-100">{t.home.suppliersText}</p>
            <div className="mt-6">
              <ButtonLink href="/register" className="ring-1 ring-white/30">
                {t.home.suppliersCta}
              </ButtonLink>
            </div>
          </div>
          <ul className="space-y-3">
            {[t.home.suppliersPoint1, t.home.suppliersPoint2, t.home.suppliersPoint3].map((point) => (
              <li key={point} className="flex items-start gap-3 rounded-2xl bg-brand-800 p-4">
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
