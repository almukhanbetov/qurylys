"use client";

import { useState } from "react";
import { brandApi } from "@/lib/api";
import { useI18n } from "@/lib/i18n/client";
import { format } from "@/lib/i18n";
import { useApiQuery } from "@/hooks/useApiQuery";
import { useSubmit } from "@/hooks/useSubmit";
import type { Brand, LegacyBrandText } from "@/types/api";
import { Button } from "@/components/ui/Button";
import { Alert, Card, Field } from "@/components/ui/Field";
import { EmptyState } from "@/components/ui/StateMessage";
import { Pagination } from "@/features/catalog/Pagination";
import { QueryState } from "@/features/account/QueryState";
import { BrandPicker } from "./BrandPicker";
import { SearchBar } from "./parts";

export function BrandsView() {
  const { t } = useI18n();
  const a = t.admin;
  const [q, setQ] = useState("");
  const [page, setPage] = useState(1);
  const result = useApiQuery(`admin-brands:${q}:${page}`, (signal) => brandApi.list(q, page, signal));
  const legacy = useApiQuery("admin-brands-legacy", (signal) => brandApi.unlinked("", signal));
  const [name, setName] = useState("");
  const { pending, error, run } = useSubmit();
  const refresh = () => {
    result.retry();
    legacy.retry();
  };

  return (
    <div className="space-y-4">
      <h1 className="text-2xl font-bold">{a.sections.brands}</h1>
      <p className="text-sm text-ink-600">{a.brands.text}</p>
      <Card title={a.brands.add}>
        <form
          className="flex flex-wrap items-end gap-3"
          onSubmit={(e) => {
            e.preventDefault();
            void run(() => brandApi.create(name)).then((ok) => {
              if (!ok) return;
              setName("");
              refresh();
            });
          }}
        >
          <div className="min-w-0 flex-1 basis-60">
            <Field label={a.brands.name} value={name} maxLength={100} required onChange={(e) => setName(e.target.value)} />
          </div>
          <Button type="submit" disabled={pending} className="h-11">{a.brands.add}</Button>
        </form>
        {error ? <div className="mt-3"><Alert>{error}</Alert></div> : null}
      </Card>
      <SearchBar
        label={a.brands.searchLabel}
        initial={q}
        onSearch={(v) => {
          setQ(v);
          setPage(1);
        }}
      />
      <QueryState result={result}>
        {(data) =>
          data.items.length === 0 ? (
            <EmptyState title={a.brands.empty} />
          ) : (
            <>
              <p className="text-sm text-ink-500">{format(a.total, { count: data.total })}</p>
              <ul className="divide-y divide-ink-100 rounded-2xl border border-ink-200 bg-white">
                {data.items.map((b) => (
                  <BrandRow key={`${b.id}:${b.version}`} brand={b} onChanged={refresh} />
                ))}
              </ul>
              <Pagination page={data.page} totalPages={data.total_pages} onChange={setPage} />
            </>
          )
        }
      </QueryState>
      <Card title={a.brands.legacy}>
        <p className="mb-4 text-sm text-ink-600">{a.brands.legacyText}</p>
        <QueryState result={legacy}>
          {(items) =>
            items.length === 0 ? (
              <p className="text-sm text-ink-500">{a.brands.legacyEmpty}</p>
            ) : (
              <ul className="divide-y divide-ink-100">
                {items.map((l) => (
                  <LegacyRow key={l.text} item={l} onChanged={refresh} />
                ))}
              </ul>
            )
          }
        </QueryState>
      </Card>
    </div>
  );
}

function BrandRow({ brand: b, onChanged }: { brand: Brand; onChanged: () => void }) {
  const { t } = useI18n();
  const a = t.admin;
  const [editing, setEditing] = useState(false);
  const [name, setName] = useState(b.name);
  const { pending, error, run } = useSubmit();
  return (
    <li className="space-y-2 p-3">
      {editing ? (
        <form
          className="flex flex-wrap items-end gap-2"
          onSubmit={(e) => {
            e.preventDefault();
            void run(() => brandApi.rename(b.id, b.version, name)).then((ok) => ok && onChanged());
          }}
        >
          <div className="min-w-0 flex-1 basis-56">
            <Field label={a.brands.name} value={name} maxLength={100} required onChange={(e) => setName(e.target.value)} />
          </div>
          <Button type="submit" disabled={pending} className="h-11">{a.save}</Button>
          <Button type="button" variant="ghost" className="h-11" onClick={() => setEditing(false)}>{a.cancel}</Button>
        </form>
      ) : (
        <div className="flex flex-wrap items-center justify-between gap-2">
          <div className="min-w-0">
            <p className="font-semibold wrap-anywhere">{b.name}</p>
            <p className="text-xs text-ink-500">{format(a.brands.products, { count: b.product_count })}</p>
          </div>
          <div className="flex flex-wrap gap-2">
            <Button variant="secondary" disabled={pending} onClick={() => setEditing(true)}>{a.edit}</Button>
            <Button
              variant="ghost"
              disabled={pending}
              className="text-red-700 hover:bg-red-50"
              onClick={() => {
                if (!window.confirm(format(a.brands.confirmDelete, { name: b.name }))) return;
                void run(() => brandApi.remove(b.id, b.version)).then((ok) => ok && onChanged());
              }}
            >
              {a.remove}
            </Button>
          </div>
        </div>
      )}
      {error ? <Alert>{error}</Alert> : null}
    </li>
  );
}

function LegacyRow({ item: l, onChanged }: { item: LegacyBrandText; onChanged: () => void }) {
  const { t } = useI18n();
  const a = t.admin;
  const [target, setTarget] = useState<{ id: number; name: string } | null>(l.match ? { id: l.match.id, name: l.match.name } : null);
  const [done, setDone] = useState<number | null>(null);
  const { pending, error, run } = useSubmit();

  function link(id: number, name: string) {
    if (!window.confirm(format(a.brands.confirmLink, { count: l.product_count, text: l.text, name }))) return;
    void run(async () => {
      const r = await brandApi.link(id, l.text);
      setDone(r.linked);
    }).then((ok) => ok && onChanged());
  }

  return (
    <li className="space-y-2 py-3">
      <p className="text-sm">
        <span className="font-semibold wrap-anywhere">«{l.text}»</span>{" "}
        <span className="text-ink-500">{format(a.brands.products, { count: l.product_count })}</span>
        {l.match ? <span className="ml-2 text-brand-800">{format(a.brands.match, { name: l.match.name })}</span> : null}
      </p>
      <div className="grid gap-2 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-end">
        <BrandPicker
          label={a.brands.linkTo}
          value={target?.id ?? null}
          current={target?.name}
          onChange={(id, name) => setTarget(id ? { id, name: name ?? "" } : null)}
          disabled={pending}
        />
        <div className="flex flex-wrap gap-2">
          <Button disabled={pending || !target} onClick={() => target && link(target.id, target.name)}>
            {a.brands.link}
          </Button>
          {l.match ? null : (
            <Button
              variant="secondary"
              disabled={pending}
              onClick={() =>
                void run(async () => {
                  const created = await brandApi.create(l.text);
                  setTarget({ id: created.id, name: l.text.trim().replace(/\s+/g, " ") });
                }).then((ok) => ok && onChanged())
              }
            >
              {a.brands.createFromText}
            </Button>
          )}
        </div>
      </div>
      {done !== null ? <Alert tone="success">{format(a.brands.linked, { count: done })}</Alert> : null}
      {error ? <Alert>{error}</Alert> : null}
    </li>
  );
}
