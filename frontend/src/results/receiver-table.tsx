import { useEffect, useRef, useState, useMemo } from "react";
import { Link } from "react-router";
import { useVirtualizer } from "@tanstack/react-virtual";
import {
  AlertCircle,
  ChevronDown,
  ChevronUp,
  Download,
  Info,
} from "lucide-react";
import { Button } from "@/ui/components/button";
import { Card } from "@/ui/components/card";
import { Input } from "@/ui/components/input";
import { Callout } from "@/ui/callout";
import { formatCoordinate, formatLevel, formatNumber } from "@/ui/format";
import { LoadingLine } from "@/ui/loading-line";
import { useReceiverTable } from "@/api/hooks";
import type { RunSummary } from "@/api/client";
import { unitFor } from "@/map/result-units";
import { buildReceiverTableCSV } from "@/model/receiver-csv";
import { summariseIndicators } from "@/results/summarise";
import { RUN_PARAM, SELECT_PARAM } from "@/map/map-params";
import { useModelStore } from "@/model/model-store";
import { m } from "@/i18n/messages";

type SortDir = "asc" | "desc";

/**
 * "2 / 3 records" — how many rows the filter kept out of how many the table
 * holds. One message carries the whole sentence rather than three fragments
 * glued in JSX: German pluralises the noun itself, so a translator has to own
 * the word next to the number. The plural follows the total, which is the
 * count the noun names; the filtered count only qualifies it.
 */
function recordCount(shown: number, total: number): string {
  return total === 1
    ? m.msg_records_count_one({ shown, total })
    : m.msg_records_count_other({ shown, total });
}

/**
 * One collator for the whole module, rather than a `localeCompare` call per
 * comparison.
 *
 * `String.prototype.localeCompare` with no options is specified to behave as
 * if a fresh collator were constructed for the call; engines cache that, but
 * the cache is not free and a 250 000-row table asks for roughly 4.5 million
 * comparisons on every sort. Hoisting it makes the construction happen once.
 *
 * Deliberately the default options. `numeric: true` would put "R2" before
 * "R10", which is what a reader expects of ids that end in digits — and which
 * is a different table from the one this component has always shown. That is a
 * behaviour change and belongs in its own commit, not in a hoist.
 */
const idCollator = new Intl.Collator();

/**
 * Every body row is exactly this tall, so `estimateSize` is not an estimate
 * and nothing has to be measured after it is painted.
 *
 * The number and the class are one fact spelled twice, because Tailwind reads
 * class names out of the source and cannot see through a template literal.
 * Change one and change the other. 29px = a 16px `text-xs` line box, 12px of
 * `py-1.5`, and the 1px bottom border.
 */
const ROW_HEIGHT_PX = 29;
const ROW_CLASS = "h-[29px] border-b last:border-0 hover:bg-muted/30";

/**
 * How the row the reader arrived on is marked.
 *
 * Paint only — a background and a foreground colour, nothing that changes the
 * box. A marked row that were one pixel taller than the others would make
 * `estimateSize` an estimate again: the virtualizer would place every row
 * after it at the wrong offset and the window would drift away from the
 * scroll position as the reader moved.
 *
 * The hover colour is re-declared over the row's own, which Tailwind would
 * otherwise emit after this one and paint over it: the mark disappearing
 * under the pointer is exactly when the reader is checking they have the
 * right row.
 *
 * The visual mark is not the whole signal. `aria-current` on the row carries
 * it to a reader who cannot see the colour, and it is `aria-current` rather
 * than `aria-selected` because the latter is not valid on a plain `<tr>`
 * outside a grid — `/results` is listed as clean in `e2e/a11y.spec.ts`, which
 * fails on a new violation.
 */
const MARKED_ROW_CLASS = `${ROW_CLASS} bg-accent text-accent-foreground hover:bg-accent`;

/**
 * How many rows beyond the viewport to keep mounted, so a fast scroll does not
 * outrun the render.
 */
const ROW_OVERSCAN = 12;

/**
 * The cap on the scroll element's height — and the whole reason the window is
 * a window.
 *
 * `overflow-auto` alone does not bound anything. Without a resolved height the
 * div grows to its content, and its content is the spacer rows, whose height
 * is the *whole* table: 250 000 rows is a 7 250 000px div. `virtual-core`
 * reads that back as the viewport, concludes every row is visible, and mounts
 * all of them — the exact DOM this component exists to avoid. The element that
 * actually scrolled in that layout was the ancestor in `pages/results.tsx`,
 * which the virtualizer is not watching.
 *
 * `vh` is resolved against the viewport rather than against a parent, so the
 * cap holds wherever the table is mounted and does not depend on an ancestor
 * chain propagating a definite height. It is a max, not a height: a table
 * short enough to fit still sizes to its rows and never scrolls.
 *
 * Written inline rather than as a Tailwind class so that it is a property of
 * the element a test can read, instead of a class name whose effect only
 * exists once a stylesheet is loaded — which, under jsdom, it is not.
 */
const TABLE_MAX_HEIGHT = "70vh";

/**
 * Where a row sends the reader: the map, with the editor already open on that
 * receiver.
 *
 * A real link and never a button that navigates. The two pages are separate
 * routes — the table is on `/results` and the map on `/model` — so this is a
 * navigation, and a button has no href to copy, no middle-click, no context
 * menu and no entry in a screen reader's links rotor. No axe rule catches that
 * substitution, so it has to be made deliberately.
 *
 * `/model` honours both parameters once and strips them (`ArrivalParams`), so a
 * Back does not re-open the editor on a receiver the reader has moved on from.
 *
 * The run travels with the receiver. Without it the map draws whichever run
 * finished last, so a row followed from an older run landed on a *different*
 * run's levels, under the id of the one that was clicked — the map names the
 * run it drew, so nothing on screen was false, but nothing said the two were
 * not the same either. The run id does not make an unselectable row selectable:
 * the link is still offered only for a row the model can open — see
 * {@link useSelectableIds}.
 */
function receiverOnMapPath(id: string, runId: string): string {
  const params = new URLSearchParams({
    [SELECT_PARAM]: id,
    [RUN_PARAM]: runId,
  });
  return `/model?${params.toString()}`;
}

/**
 * The ids `/model` can select, so that the ones it cannot are not offered as
 * links.
 *
 * A run's rows are not always model objects. In the default `auto-grid`
 * receiver mode the CLI generates the receivers itself and names them
 * `grid-000000`…; nothing of that kind is in the model store, and
 * `SelectRequest` hands the id to `FeatureEditor`, which looks it up among the
 * features and the explicit receivers and renders nothing when it is neither.
 * A link for such a row navigates to the map, strips the parameter and opens
 * nothing — a promise the page cannot keep. The id is then plain text.
 *
 * One namespace across features and receivers, which is what `mergeModel`
 * already assumes. Built from the two arrays rather than selected as a set:
 * a selector returning a fresh `Set` is a new value on every store read.
 */
function useSelectableIds(): Set<string> {
  const features = useModelStore((s) => s.features);
  const receivers = useModelStore((s) => s.receivers);

  return useMemo(
    () =>
      new Set([
        ...features.map((feature) => feature.id),
        ...receivers.map((receiver) => receiver.id),
      ]),
    [features, receivers],
  );
}

/** The sort indicator in a column header: filled for the sorted column. */
function SortIcon({
  col,
  sortCol,
  sortDir,
}: {
  col: string;
  sortCol: string;
  sortDir: SortDir;
}) {
  if (sortCol !== col)
    return <ChevronUp aria-hidden="true" className="h-3 w-3 opacity-30" />;
  return sortDir === "asc" ? (
    <ChevronUp aria-hidden="true" className="h-3 w-3" />
  ) : (
    <ChevronDown aria-hidden="true" className="h-3 w-3" />
  );
}

// The level formatter wants a unit; a table that names none falls back to
// the bare number rather than printing a dangling space.
function level(value: number, unit: string): string {
  return unit === "" ? formatNumber(value) : formatLevel(value, unit);
}

// The unit comes off the indicator, not off the table: `beb-exposure` heads
// two decibel columns and six count columns in the same row of `<th>`s.
function columnLabel(
  col: string,
  indicators: string[],
  units: Record<string, string> | undefined,
): string {
  if (col === "height_m") return m.table_header_height_m();

  if (indicators.includes(col)) {
    const unit = unitFor(units, col);
    if (unit !== "") return `${col} (${unit})`;
  }

  return col;
}

export function ReceiversTab({
  run,
  receiverId = null,
}: {
  run: RunSummary;
  /**
   * The receiver the reader arrived on from the map, scrolled to and marked.
   *
   * Handed down rather than read from the URL here: `/results` strips the
   * parameter as soon as it has been honoured, and the mark outlives it.
   */
  receiverId?: string | null;
}) {
  const artifact = run.artifacts.find(
    (a) => a.kind === "run.result.receiver_table_json",
  );

  const { data, isLoading, error } = useReceiverTable(artifact?.id ?? null);

  const [filter, setFilter] = useState("");
  const [sortCol, setSortCol] = useState<string>("id");
  const [sortDir, setSortDir] = useState<SortDir>("asc");

  const selectableIds = useSelectableIds();

  const indicators = useMemo(() => data?.indicator_order ?? [], [data]);
  const units = data?.units;

  const summaryCards = useMemo(
    () => (data ? summariseIndicators(data.records, indicators) : []),
    [data, indicators],
  );

  const filteredRecords = useMemo(() => {
    if (!data) return [];
    const q = filter.toLowerCase();
    return data.records.filter((r) => r.id.toLowerCase().includes(q));
  }, [data, filter]);

  const sortedRecords = useMemo(() => {
    const copy = [...filteredRecords];
    copy.sort((a, b) => {
      let av: string | number;
      let bv: string | number;
      if (sortCol === "id") {
        av = a.id;
        bv = b.id;
      } else if (sortCol === "x") {
        av = a.x;
        bv = b.x;
      } else if (sortCol === "y") {
        av = a.y;
        bv = b.y;
      } else if (sortCol === "height_m") {
        av = a.height_m;
        bv = b.height_m;
      } else {
        av = a.values[sortCol] ?? 0;
        bv = b.values[sortCol] ?? 0;
      }
      if (typeof av === "string" && typeof bv === "string") {
        return sortDir === "asc"
          ? idCollator.compare(av, bv)
          : idCollator.compare(bv, av);
      }
      const an = av as number;
      const bn = bv as number;
      return sortDir === "asc" ? an - bn : bn - an;
    });
    return copy;
  }, [filteredRecords, sortCol, sortDir]);

  /*
   * The window, not the table. `sortedRecords` stays the whole filtered and
   * sorted array — the CSV download is built from it, and it is what the
   * record count counts — while only the rows near the viewport are mounted.
   *
   * The scroll element is the bordered `div` below, which this component owns.
   * Both hooks sit above the early returns, where the rules of hooks need
   * them, so the virtualizer exists even on the renders that show a callout
   * instead of a table.
   */
  const scrollRef = useRef<HTMLDivElement>(null);
  const rowVirtualizer = useVirtualizer({
    count: sortedRecords.length,
    getScrollElement: () => scrollRef.current,
    estimateSize: () => ROW_HEIGHT_PX,
    overscan: ROW_OVERSCAN,
  });

  /*
   * Takes the reader to the row the map sent them to.
   *
   * The index is found in `sortedRecords`, not in `data.records`: the reader
   * can have sorted the table by any column and the virtualizer addresses the
   * list it is actually windowing, so an index from the unsorted array would
   * scroll to a different row — or, on a 250 000-row table, to somewhere in
   * the middle of nowhere.
   *
   * **The `-1` case is decided, not tolerated.** A row the filter is hiding is
   * still a row this run computed, and the honest answer is to show it: the
   * filter is cleared and this effect runs again over the widened list. A
   * silent no-op would leave the reader looking at a table that visibly does
   * not contain what they clicked, with nothing on screen saying why. An id
   * the *table* does not hold is a different thing — there is nothing to
   * reveal and clearing the filter would throw away the reader's state for
   * nothing — so that case is marked handled and left alone.
   *
   * Guarded by a ref on the id rather than by its dependencies, because
   * `sortedRecords` is in them: the effect has to re-run when the list changes
   * (the table arrives asynchronously, and the filter above widens it), but it
   * must act exactly once per arrival. A reader who lands on a row and then
   * filters it away meant to, and a second run of this would snap their filter
   * back.
   */
  const arrivedRef = useRef<string | null>(null);
  useEffect(() => {
    if (receiverId === null) return;
    if (arrivedRef.current === receiverId) return;
    // Nothing has been read yet, so "the table does not hold this id" is not
    // an answer this effect is entitled to give. Marking the arrival handled
    // here would be permanent: the guard above then refuses the run that
    // happens once the table arrives, and a row outside the initial window
    // stays invisible under a URL that has already been stripped.
    if (data === undefined) return;

    const index = sortedRecords.findIndex((record) => record.id === receiverId);
    if (index === -1) {
      if (data.records.some((record) => record.id === receiverId)) {
        setFilter("");
      } else {
        arrivedRef.current = receiverId;
      }
      return;
    }

    arrivedRef.current = receiverId;
    // Centred rather than `"start"`: a row pinned to the top edge reads as
    // the top of the table, and the rows around it are the context that says
    // it is not.
    rowVirtualizer.scrollToIndex(index, { align: "center" });
  }, [receiverId, sortedRecords, data, rowVirtualizer]);

  function toggleSort(col: string) {
    if (sortCol === col) {
      setSortDir((d) => (d === "asc" ? "desc" : "asc"));
    } else {
      setSortCol(col);
      setSortDir("asc");
    }
  }

  // Raw values, not the locale-formatted ones: the CSV is for other tools.
  // The bytes come from the shared builder, so a browser download and the CLI's
  // receivers.csv are the same file for the same table.
  function downloadCSV() {
    if (!data) return;
    const csv = buildReceiverTableCSV({
      indicator_order: indicators,
      records: sortedRecords,
    });
    const blob = new Blob([csv], { type: "text/csv" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = "receivers.csv";
    a.click();
    URL.revokeObjectURL(url);
  }

  if (!artifact) {
    return (
      <Callout variant="neutral" icon={Info}>
        {m.msg_no_receiver_artifacts()}
      </Callout>
    );
  }

  if (isLoading) {
    return <LoadingLine text={m.status_loading_receiver_table()} />;
  }

  if (error || !data) {
    return (
      <Callout variant="destructive" icon={AlertCircle}>
        {m.error_load_receiver_table()}
      </Callout>
    );
  }

  const columns = ["id", "x", "y", "height_m", ...indicators];

  const virtualRows = rowVirtualizer.getVirtualItems();
  const spacerAbove = virtualRows.at(0)?.start ?? 0;
  const lastEnd = virtualRows.at(-1)?.end;
  const spacerBelow =
    lastEnd === undefined ? 0 : rowVirtualizer.getTotalSize() - lastEnd;

  /*
   * What the table says about its own size.
   *
   * With the rows windowed the DOM no longer holds the answer, so a screen
   * reader asking "how big is this table?" would be told it has twelve rows.
   * `aria-rowcount` is the whole filtered view plus the header, and every
   * rendered row carries the `aria-rowindex` it would have if all of them were
   * there — the header being 1, so a record's index is its offset plus 2.
   *
   * The "nothing matched" row is a row like any other, which is why an empty
   * view still counts as one body row rather than none.
   */
  const bodyRowCount = sortedRecords.length === 0 ? 1 : sortedRecords.length;

  return (
    <div className="flex flex-col gap-4">
      {/* Indicator summary cards */}
      {summaryCards.length > 0 ? (
        <div className="flex flex-wrap gap-3">
          {summaryCards.map(({ ind, min, max, mean }) => {
            // The messages carry the bare term; the `dt` below punctuates
            // it. `locale-parity.test.ts` refuses a colon typed into either
            // catalogue, so the colon belongs here and nowhere else.
            const stats: Array<[string, number]> = [
              [m.label_min(), min],
              [m.label_max(), max],
              [m.label_mean(), mean],
            ];
            return (
              <Card key={ind} className="min-w-36 p-3">
                <p className="font-mono text-xs font-semibold text-muted-foreground">
                  {ind}
                </p>
                <dl className="mt-1 space-y-0.5 text-xs">
                  {stats.map(([label, value]) => (
                    <div key={label} className="flex gap-1">
                      <dt className="text-muted-foreground">{label}:</dt>
                      <dd className="font-medium tabular-nums">
                        {level(value, unitFor(units, ind))}
                      </dd>
                    </div>
                  ))}
                </dl>
              </Card>
            );
          })}
        </div>
      ) : null}

      {/* Filter + Download */}
      <div className="flex items-center gap-3">
        <Input
          className="h-8 w-64 text-xs"
          aria-label={m.label_filter_receiver_id()}
          placeholder={m.label_filter_receiver_id()}
          value={filter}
          onChange={(e) => {
            setFilter(e.target.value);
          }}
        />
        <span className="text-xs text-muted-foreground">
          {recordCount(sortedRecords.length, data.records.length)}
        </span>
        <div className="ml-auto">
          <Button variant="outline" size="sm" onClick={downloadCSV}>
            <Download aria-hidden="true" className="mr-1.5 h-3.5 w-3.5" />
            {m.action_download_csv()}
          </Button>
        </div>
      </div>

      {/* Table */}
      <div
        ref={scrollRef}
        style={{ maxHeight: TABLE_MAX_HEIGHT }}
        className="overflow-auto rounded-md border"
      >
        <table className="w-full text-xs" aria-rowcount={bodyRowCount + 1}>
          <thead>
            <tr aria-rowindex={1} className="border-b bg-muted/50">
              {columns.map((col) => (
                <th
                  key={col}
                  scope="col"
                  aria-sort={
                    sortCol === col
                      ? sortDir === "asc"
                        ? "ascending"
                        : "descending"
                      : undefined
                  }
                  className="whitespace-nowrap px-3 py-2 text-left font-semibold text-muted-foreground"
                >
                  <button
                    type="button"
                    onClick={() => {
                      toggleSort(col);
                    }}
                    className="inline-flex items-center gap-1 rounded-sm hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                  >
                    {columnLabel(col, indicators, units)}
                    <SortIcon col={col} sortCol={sortCol} sortDir={sortDir} />
                  </button>
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {/*
              The scrolled-past rows are a spacer row, not an absolutely
              positioned window. Absolute positioning forces `display: flex` on
              every `<tr>`, which is exactly the point at which the browser
              stops treating this as a table: `scope="col"` and `aria-sort` on
              the headers above describe a grid that would no longer exist.
              A `<tr>` holding one tall `<td colSpan>` keeps the native
              semantics and costs two nodes. Each is rendered only when it has
              a height, so a fully visible table has neither and its last row
              is still `:last-child`.
            */}
            {spacerAbove > 0 ? (
              <tr aria-hidden="true">
                <td colSpan={columns.length} style={{ height: spacerAbove }} />
              </tr>
            ) : null}
            {virtualRows.map((virtualRow) => {
              const r = sortedRecords[virtualRow.index];
              if (r === undefined) return null;
              return (
                <tr
                  key={r.id}
                  aria-rowindex={virtualRow.index + 2}
                  aria-current={r.id === receiverId ? "true" : undefined}
                  className={r.id === receiverId ? MARKED_ROW_CLASS : ROW_CLASS}
                >
                  <td className="px-3 py-1.5 font-mono">
                    {selectableIds.has(r.id) ? (
                      <Link
                        to={receiverOnMapPath(r.id, run.id)}
                        aria-label={m.action_show_receiver_on_map({ id: r.id })}
                        className="rounded-sm underline decoration-dotted underline-offset-2 hover:decoration-solid focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                      >
                        {r.id}
                      </Link>
                    ) : (
                      r.id
                    )}
                  </td>
                  <td className="px-3 py-1.5 tabular-nums">
                    {formatCoordinate(r.x)}
                  </td>
                  <td className="px-3 py-1.5 tabular-nums">
                    {formatCoordinate(r.y)}
                  </td>
                  <td className="px-3 py-1.5 tabular-nums">
                    {formatNumber(r.height_m)}
                  </td>
                  {indicators.map((ind) => (
                    <td key={ind} className="px-3 py-1.5 tabular-nums">
                      {formatNumber(r.values[ind] ?? 0)}
                    </td>
                  ))}
                </tr>
              );
            })}
            {spacerBelow > 0 ? (
              <tr aria-hidden="true">
                <td colSpan={columns.length} style={{ height: spacerBelow }} />
              </tr>
            ) : null}
            {sortedRecords.length === 0 ? (
              <tr aria-rowindex={2}>
                <td
                  colSpan={columns.length}
                  className="px-3 py-6 text-center text-muted-foreground"
                >
                  {m.msg_no_records_match_filter()}
                </td>
              </tr>
            ) : null}
          </tbody>
        </table>
      </div>
    </div>
  );
}
