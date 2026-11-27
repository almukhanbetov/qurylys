"use client";

import { useState } from "react";
import { categoryAdminApi } from "@/lib/api";
import { useI18n } from "@/lib/i18n/client";
import { format, pick } from "@/lib/i18n";
import { useApiQuery } from "@/hooks/useApiQuery";
import { useSubmit } from "@/hooks/useSubmit";
import type { AdminCategory, CategoryInput } from "@/types/api";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Alert, Card, Field } from "@/components/ui/Field";
import { EmptyState } from "@/components/ui/StateMessage";
import { QueryState } from "@/features/account/QueryState";
import { Select } from "./parts";

type Row = AdminCategory & { depth: number };

/** Depth-first order of the tree; siblings keep the API order (sort_order, id). */
function ordered(all: AdminCategory[]): Row[] {
  const children = new Map<number | null, AdminCategory[]>();
  for (const c of all) children.set(c.parent_id, [...(children.get(c.parent_id) ?? []), c]);
  const out: Row[] = [];
  const walk = (parent: number | null, depth: number) => {
    for (const c of children.get(parent) ?? []) {
      out.push({ ...c, depth });
      walk(c.id, depth + 1);
    }
  };
  walk(null, 0);
  return out;
}

/** The category and all its descendants: not allowed as its new parent. */
function subtree(rows: Row[], id: number): Set<number> {
  const out = new Set([id]);
  for (const r of rows) if (r.parent_id !== null && out.has(r.parent_id)) out.add(r.id);
  return out;
}

type Editing = { mode: "create"; parent: number | null } | { mode: "edit"; category: Row };

export function CategoriesView() {
  const { t, locale } = useI18n();
  const a = t.admin;
  const result = useApiQuery("admin-categories", (signal) => categoryAdminApi.list(signal));
  const [editing, setEditing] = useState<Editing | null>(null);
  const { pending, error, run } = useSubmit();

  return (
    <div className="space-y-4">
      <h1 className="text-2xl font-bold">{a.sections.categories}</h1>
      <p className="text-sm text-ink-600">{a.categories.text}</p>
      <Alert tone="info">{a.categories.fieldRules}</Alert>
      <QueryState result={result}>
        {(all) => {
          const rows = ordered(all);
          return (
            <>
              {editing ? (
                <CategoryForm
                  key={editing.mode === "edit" ? `e${editing.category.id}:${editing.category.version}` : `c${editing.parent}`}
                  rows={rows}
                  editing={editing}
                  onDone={() => {
                    setEditing(null);
                    result.retry();
                  }}
                  onCancel={() => setEditing(null)}
                />
              ) : (
                <Button onClick={() => setEditing({ mode: "create", parent: null })}>{a.categories.addRoot}</Button>
              )}
              {error ? <Alert>{error}</Alert> : null}
              {rows.length === 0 ? (
                <EmptyState title={a.categories.empty} />
              ) : (
                <ul className="divide-y divide-ink-100 rounded-2xl border border-ink-200 bg-white">
                  {rows.map((c) => (
                    <li key={c.id} className="flex flex-wrap items-center justify-between gap-3 p-3" style={{ paddingInlineStart: `${0.75 + Math.min(c.depth, 6) * 1.25}rem` }}>
                      <div className="min-w-0 flex-1 basis-56">
                        <p className="font-semibold wrap-anywhere">
                          {pick(locale, c.name_kk, c.name_ru)}{" "}
                          {c.is_active ? null : <Badge tone="muted">{a.categories.inactive}</Badge>}
                        </p>
                        <p className="text-xs text-ink-500 wrap-anywhere">
                          {c.slug} · {format(a.categories.counts, { products: c.product_count, children: c.child_count, requests: c.request_count })}
                        </p>
                      </div>
                      <div className="flex flex-wrap gap-2">
                        <Button variant="ghost" disabled={pending} onClick={() => setEditing({ mode: "create", parent: c.id })}>
                          {a.categories.addChild}
                        </Button>
                        <Button variant="secondary" disabled={pending} onClick={() => setEditing({ mode: "edit", category: c })}>
                          {a.edit}
                        </Button>
                        <Button
                          variant="ghost"
                          disabled={pending}
                          className="text-red-700 hover:bg-red-50"
                          onClick={() => {
                            if (!window.confirm(format(a.categories.confirmDelete, { name: pick(locale, c.name_kk, c.name_ru) }))) return;
                            void run(() => categoryAdminApi.remove(c.id, c.version)).then((ok) => ok && result.retry());
                          }}
                        >
                          {a.remove}
                        </Button>
                      </div>
                    </li>
                  ))}
                </ul>
              )}
            </>
          );
        }}
      </QueryState>
    </div>
  );
}

function CategoryForm({ rows, editing, onDone, onCancel }: { rows: Row[]; editing: Editing; onDone: () => void; onCancel: () => void }) {
  const { t, locale } = useI18n();
  const a = t.admin;
  const c = editing.mode === "edit" ? editing.category : null;
  const [f, setF] = useState<CategoryInput>(
    c
      ? { parent_id: c.parent_id, name_kk: c.name_kk, name_ru: c.name_ru, sort_order: c.sort_order }
      : { parent_id: editing.mode === "create" ? editing.parent : null, slug: "", name_kk: "", name_ru: "", sort_order: 0 },
  );
  const { pending, error, run } = useSubmit();
  const blocked = c ? subtree(rows, c.id) : new Set<number>();

  return (
    <Card title={c ? a.categories.editTitle : a.categories.newTitle}>
      <form
        className="grid gap-3 sm:grid-cols-2"
        onSubmit={(e) => {
          e.preventDefault();
          void run(() => (c ? categoryAdminApi.update(c.id, c.version, f) : categoryAdminApi.create(f))).then((ok) => ok && onDone());
        }}
      >
        <div className="sm:col-span-2">
          <Select label={a.categories.parent} value={f.parent_id === null ? "" : String(f.parent_id)} onChange={(v) => setF({ ...f, parent_id: v ? Number(v) : null })}>
            <option value="">{a.categories.root}</option>
            {rows.map((r) => (
              <option key={r.id} value={r.id} disabled={blocked.has(r.id)}>
                {"  ".repeat(r.depth) + pick(locale, r.name_kk, r.name_ru)}
              </option>
            ))}
          </Select>
        </div>
        {c ? (
          <Field label={a.categories.slug} value={c.slug} disabled readOnly />
        ) : (
          <Field
            label={a.categories.slug}
            hint={a.categories.slugHint}
            value={f.slug ?? ""}
            maxLength={80}
            required
            pattern="[a-z0-9]+(-[a-z0-9]+)*"
            onChange={(e) => setF({ ...f, slug: e.target.value.toLowerCase() })}
          />
        )}
        <Field
          label={a.categories.sortOrder}
          type="number"
          min={-100000}
          max={100000}
          value={f.sort_order}
          onChange={(e) => setF({ ...f, sort_order: Number(e.target.value) || 0 })}
        />
        <Field label={t.catalogRequests.nameKk} value={f.name_kk} maxLength={200} required onChange={(e) => setF({ ...f, name_kk: e.target.value })} />
        <Field label={t.catalogRequests.nameRu} value={f.name_ru} maxLength={200} required onChange={(e) => setF({ ...f, name_ru: e.target.value })} />
        {error ? <div className="sm:col-span-2"><Alert>{error}</Alert></div> : null}
        <div className="flex flex-wrap gap-2 sm:col-span-2">
          <Button type="submit" disabled={pending}>{pending ? t.common.loading : a.save}</Button>
          <Button type="button" variant="ghost" onClick={onCancel}>{a.cancel}</Button>
        </div>
      </form>
    </Card>
  );
}
