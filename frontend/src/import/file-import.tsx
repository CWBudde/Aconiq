import { useCallback, useRef } from "react";
import { FileInput } from "lucide-react";
import { Button } from "@/ui/components/button";
import { PageHeader } from "@/ui/page-header";
import type { GeoJSONFeatureCollection } from "@/model/types";
import { m } from "@/i18n/messages";

/**
 * The file half of the import wizard: a drop zone, a file picker, and the two
 * shapes a chosen file can fail on before anything reads it as a model.
 *
 * It hands a parsed `FeatureCollection` up and decides nothing about it — what
 * the collection becomes is the page's step machine's business.
 */
export function FileImport({
  onCollection,
  onError,
}: {
  onCollection: (collection: GeoJSONFeatureCollection) => void;
  onError: (message: string | null) => void;
}) {
  const fileRef = useRef<HTMLInputElement>(null);

  const handleFile = useCallback(
    async (file: File) => {
      onError(null);
      try {
        const text = await file.text();
        const parsed = JSON.parse(text) as Record<string, unknown>;
        if (
          parsed["type"] !== "FeatureCollection" ||
          !Array.isArray(parsed["features"])
        ) {
          onError(m.msg_geojson_error_invalid());
          return;
        }
        onCollection(parsed as unknown as GeoJSONFeatureCollection);
      } catch {
        onError(m.msg_geojson_error_parse());
      }
    },
    [onCollection, onError],
  );

  const handleDrop = useCallback(
    (e: React.DragEvent) => {
      e.preventDefault();
      const file = e.dataTransfer.files[0];
      if (file) void handleFile(file);
    },
    [handleFile],
  );

  const handleInputChange = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) => {
      const file = e.target.files?.[0];
      if (file) void handleFile(file);
    },
    [handleFile],
  );

  return (
    <div
      className="flex flex-col items-center gap-4 rounded-lg border-2 border-dashed p-12 text-center"
      onDrop={handleDrop}
      onDragOver={(e) => {
        e.preventDefault();
      }}
    >
      <FileInput className="size-10 text-muted-foreground" aria-hidden="true" />
      <PageHeader
        className="justify-center text-center"
        title={m.heading_import_geojson()}
        description={m.msg_drag_or_click()}
      />
      <Button
        onClick={() => {
          fileRef.current?.click();
        }}
      >
        {m.action_choose_file()}
      </Button>
      <input
        ref={fileRef}
        type="file"
        accept=".geojson,.json"
        className="hidden"
        onChange={handleInputChange}
      />
    </div>
  );
}
