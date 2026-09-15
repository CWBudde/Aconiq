import { useState, useMemo } from "react";
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
import { buildReceiverTableCSV } from "@/model/receiver-csv";
import { summariseIndicators } from "@/results/summarise";
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

function columnLabel(col: string, indicators: string[], unit: string): string {
  if (col === "height_m") return m.table_header_height_m();
  if (indicators.includes(col) && unit !== "") return `${col} (${unit})`;
  return col;
}

export function ReceiversTab({ run }: { run: RunSummary }) {
  const artifact = run.artifacts.find(
    (a) => a.kind === "run.result.receiver_table_json",
  );

  const { data, isLoading, error } = useReceiverTable(artifact?.id ?? null);

  const [filter, setFilter] = useState("");
  const [sortCol, setSortCol] = useState<string>("id");
  const [sortDir, setSortDir] = useState<SortDir>("asc");

  const indicators = useMemo(() => data?.indicator_order ?? [], [data]);
  const unit = data?.unit ?? "";

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
                        {level(value, unit)}
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
      <div className="overflow-auto rounded-md border">
        <table className="w-full text-xs">
          <thead>
            <tr className="border-b bg-muted/50">
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
                    {columnLabel(col, indicators, unit)}
                    <SortIcon col={col} sortCol={sortCol} sortDir={sortDir} />
                  </button>
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {sortedRecords.map((r) => (
              <tr
                key={r.id}
                className="border-b last:border-0 hover:bg-muted/30"
              >
                <td className="px-3 py-1.5 font-mono">{r.id}</td>
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
            ))}
            {sortedRecords.length === 0 ? (
              <tr>
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
