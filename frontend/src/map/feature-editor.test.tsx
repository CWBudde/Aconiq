import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { MemoryRouter } from "react-router";
import type { ReceiverTable, RunSummary } from "@/api/client";
import { FeatureEditor } from "./feature-editor";
import { useModelStore } from "@/model/model-store";
import {
  RLS19_JUNCTION_TYPES,
  RLS19_SURFACE_TYPES,
} from "@/model/source-acoustics";
import {
  PROP_PARKING_TYPE,
  RLS19_PARKING_FACILITY_TYPES,
  RLS19_PARKING_LOT_TYPES,
} from "@/model/rls19-parking";
import {
  BIMSCHV16_AREA_CATEGORIES,
  bimschv16AreaCategoryLabel,
  PROP_BIMSCHV16_AREA_CATEGORY,
} from "@/model/bimschv16";
import type { ModelFeature, ModelReceiver } from "@/model/types";
import { validationIssueText } from "@/model/validation-message";
import { MAIN_CONTENT_ID } from "@/ui/main-content";
import { getLocale, overwriteGetLocale, type Locale } from "@/i18n/runtime";
import { m } from "@/i18n/messages";

/**
 * The one hook the editor reaches for, and only when it is handed a run: the
 * link into the results table is offered for an id that run's table actually
 * holds, which is a question only the table can answer.
 */
const api = vi.hoisted(() => {
  const value: {
    table: ReceiverTable | undefined;
    /** Every artifact id `useReceiverTable` was asked for, `null` included. */
    askedFor: (string | null)[];
  } = { table: undefined, askedFor: [] };
  return value;
});

vi.mock("@/api/hooks", () => ({
  useReceiverTable: (artifactId: string | null) => {
    api.askedFor.push(artifactId);
    return {
      data: artifactId === null ? undefined : api.table,
      isLoading: false,
      error: null,
    };
  },
}));

const originalGetLocale = getLocale;

function useLocale(locale: Locale) {
  overwriteGetLocale(() => locale);
}

afterEach(() => {
  overwriteGetLocale(originalGetLocale);
});

/**
 * The editor is the only place a feature or a receiver can be deleted, and it
 * is not on the map's own surface — so nothing here needs a WebGL context. The
 * panel is plain DOM.
 */

const source: ModelFeature = {
  id: "src-1",
  kind: "source",
  sourceType: "point",
  geometry: { type: "Point", coordinates: [10, 51] },
};

const receiver: ModelReceiver = {
  id: "rcv-1",
  heightM: 4,
  geometry: { type: "Point", coordinates: [10.01, 51.01] },
};

function deleteButton(): HTMLElement {
  return screen.getByRole("button", { name: m.action_delete_feature() });
}

function confirmDelete() {
  fireEvent.click(
    within(screen.getByRole("alertdialog")).getByRole("button", {
      name: m.action_delete_feature(),
    }),
  );
}

beforeEach(() => {
  useModelStore.getState().reset();
});

describe("FeatureEditor deletion", () => {
  it("asks before deleting a feature, and deletes nothing until answered", () => {
    useModelStore.getState().addFeature(source);
    render(<FeatureEditor featureId="src-1" onClose={vi.fn()} />);

    fireEvent.click(deleteButton());

    expect(screen.getByRole("alertdialog")).toBeInTheDocument();
    expect(useModelStore.getState().features).toHaveLength(1);
  });

  it("opens the same confirmation on Del", () => {
    // The Phase D keyboard path: Del deletes the edited feature. It goes
    // through the button's own dialog rather than straight to `removeFeature`,
    // so there is one delete path and the sentence about undo is not skipped
    // by whoever reaches for the keyboard.
    useModelStore.getState().addFeature(source);
    render(<FeatureEditor featureId="src-1" onClose={vi.fn()} />);

    fireEvent.keyDown(window, { key: "Delete" });

    expect(screen.getByRole("alertdialog")).toBeInTheDocument();
    expect(useModelStore.getState().features).toHaveLength(1);

    confirmDelete();
    expect(useModelStore.getState().features).toHaveLength(0);
  });

  it("deletes the edited receiver on Del as well", () => {
    useModelStore.getState().addReceiver(receiver);
    render(<FeatureEditor featureId="rcv-1" onClose={vi.fn()} />);

    fireEvent.keyDown(window, { key: "Delete" });
    confirmDelete();

    expect(useModelStore.getState().receivers).toHaveLength(0);
  });

  it("leaves Del to the text field the user is typing in", () => {
    // `useGlobalShortcut` bows out of text controls, which is what keeps Del
    // editing a number in the height field instead of deleting the feature
    // the field belongs to.
    useModelStore.getState().addFeature({ ...source, kind: "barrier" });
    render(<FeatureEditor featureId="src-1" onClose={vi.fn()} />);
    const height = screen.getByLabelText(m.label_height_m());

    fireEvent.keyDown(height, { key: "Delete" });

    expect(screen.queryByRole("alertdialog")).toBeNull();
    expect(useModelStore.getState().features).toHaveLength(1);
  });

  it("names what is being deleted, and says undo can bring it back", () => {
    // The panel has no undo control of its own and the undo bar is at the
    // other end of the workspace, so the confirmation is where that is said.
    useModelStore.getState().addFeature(source);
    render(<FeatureEditor featureId="src-1" onClose={vi.fn()} />);

    fireEvent.click(deleteButton());

    const dialog = screen.getByRole("alertdialog");
    expect(dialog).toHaveTextContent("src-1");
    expect(dialog).toHaveTextContent(m.option_source());
  });

  it("deletes the feature and closes the panel once confirmed", () => {
    const onClose = vi.fn();
    useModelStore.getState().addFeature(source);
    render(<FeatureEditor featureId="src-1" onClose={onClose} />);

    fireEvent.click(deleteButton());
    confirmDelete();

    expect(useModelStore.getState().features).toHaveLength(0);
    expect(onClose).toHaveBeenCalled();
  });

  it("keeps the feature when the confirmation is cancelled", () => {
    const onClose = vi.fn();
    useModelStore.getState().addFeature(source);
    render(<FeatureEditor featureId="src-1" onClose={onClose} />);

    fireEvent.click(deleteButton());
    fireEvent.click(
      within(screen.getByRole("alertdialog")).getByRole("button", {
        name: m.action_cancel(),
      }),
    );

    expect(useModelStore.getState().features).toHaveLength(1);
    expect(onClose).not.toHaveBeenCalled();
  });

  it("asks before deleting a receiver too", () => {
    // The receiver panel used to carry its own delete button, so it had its
    // own path past the confirmation.
    useModelStore.getState().addReceiver(receiver);
    render(<FeatureEditor featureId="rcv-1" onClose={vi.fn()} />);

    fireEvent.click(deleteButton());

    expect(screen.getByRole("alertdialog")).toHaveTextContent("rcv-1");
    expect(useModelStore.getState().receivers).toHaveLength(1);

    confirmDelete();
    expect(useModelStore.getState().receivers).toHaveLength(0);
  });

  it("puts focus back in the content when the panel deletes itself", async () => {
    // Confirming closes the panel, taking the Delete button with it, so Radix
    // restores focus to an element that is gone and it lands on `<body>`. The
    // e2e axe baseline does not cover the map route, and no rule covers this.
    useModelStore.getState().addFeature(source);
    render(
      <div id={MAIN_CONTENT_ID} tabIndex={-1}>
        <FeatureEditor featureId="src-1" onClose={vi.fn()} />
      </div>,
    );

    fireEvent.click(deleteButton());
    confirmDelete();

    await waitFor(() => {
      expect(document.activeElement?.id).toBe(MAIN_CONTENT_ID);
    });
  });

  it("does not move focus when the confirmation is cancelled", () => {
    useModelStore.getState().addFeature(source);
    render(
      <div id={MAIN_CONTENT_ID} tabIndex={-1}>
        <FeatureEditor featureId="src-1" onClose={vi.fn()} />
      </div>,
    );

    fireEvent.click(deleteButton());
    fireEvent.click(
      within(screen.getByRole("alertdialog")).getByRole("button", {
        name: m.action_cancel(),
      }),
    );

    // Only that this component does not hijack it: where Radix's own
    // restoration goes is Radix's business, and jsdom never gave the trigger
    // focus to restore.
    expect(document.activeElement?.id).not.toBe(MAIN_CONTENT_ID);
    expect(useModelStore.getState().features).toHaveLength(1);
  });

  it("is undoable after the confirmation, which is what the wording promises", () => {
    useModelStore.getState().addFeature(source);
    render(<FeatureEditor featureId="src-1" onClose={vi.fn()} />);

    fireEvent.click(deleteButton());
    confirmDelete();
    useModelStore.getState().undo();

    expect(useModelStore.getState().features).toHaveLength(1);
  });
});

/**
 * Everything below the deletion suite: the panel shell, the per-kind fields
 * and the RLS-19 source-acoustics surface, none of which had a test.
 *
 * These go through the real `model-store` and the real `source-acoustics`
 * helpers rather than stubs. The property helpers are where this editor's
 * behaviour actually lives — aliasing, the `_inferred` flags, and the
 * difference between "zero" and "unset" — and a stub would let the editor and
 * the helpers agree with each other while disagreeing with the model file.
 */

const lineSource: ModelFeature = {
  id: "road-1",
  kind: "source",
  sourceType: "line",
  geometry: {
    type: "LineString",
    coordinates: [
      [10, 51],
      [10.01, 51.01],
    ],
  },
};

const building: ModelFeature = {
  id: "bld-1",
  kind: "building",
  heightM: 12,
  geometry: {
    type: "Polygon",
    coordinates: [
      [
        [10, 51],
        [10.01, 51],
        [10.01, 51.01],
        [10, 51],
      ],
    ],
  },
};

const barrier: ModelFeature = {
  id: "bar-1",
  kind: "barrier",
  heightM: 3,
  geometry: {
    type: "LineString",
    coordinates: [
      [10, 51],
      [10.01, 51],
    ],
  },
};

function edit(feature: ModelFeature) {
  useModelStore.getState().addFeature(feature);
  render(<FeatureEditor featureId={feature.id} onClose={vi.fn()} />);
}

/**
 * Number fields are addressed by their input id rather than by label text:
 * "Cars" labels three of them (speed, day traffic, night traffic) and only the
 * id says which. Resolving through the id also checks the `htmlFor` pairing,
 * which is the field's only accessible name.
 */
function numberField(featureId: string, propertyKey: string): HTMLInputElement {
  const element = document.getElementById(`${featureId}-${propertyKey}`);
  if (!(element instanceof HTMLInputElement)) {
    throw new Error(`no number field rendered for ${propertyKey}`);
  }
  return element;
}

/**
 * A select's Radix trigger, resolved through the id its label points at — the
 * same pairing the number fields use, and the field's only accessible name.
 */
function selectField(featureId: string, propertyKey: string): HTMLElement {
  const element = document.getElementById(`${featureId}-${propertyKey}`);
  if (!(element instanceof HTMLElement)) {
    throw new Error(`no select rendered for ${propertyKey}`);
  }
  return element;
}

function type(input: HTMLElement, value: string) {
  fireEvent.change(input, { target: { value } });
  fireEvent.blur(input);
}

function storedProperties(id: string): Record<string, unknown> {
  return useModelStore.getState().getFeatureById(id)?.properties ?? {};
}

describe("FeatureEditor shell", () => {
  it("renders nothing for an id that is in neither collection", () => {
    // The map clears its selection by passing an id that has just been
    // deleted; an empty panel is correct, a crash is not.
    const { container } = render(
      <FeatureEditor featureId="gone" onClose={vi.fn()} />,
    );
    expect(container).toBeEmptyDOMElement();
  });

  it("renders nothing when no feature is selected", () => {
    const { container } = render(
      <FeatureEditor featureId={null} onClose={vi.fn()} />,
    );
    expect(container).toBeEmptyDOMElement();
  });

  it("shows the id and the geometry type it is editing", () => {
    // Two features of the same kind are otherwise indistinguishable in this
    // panel, and the geometry type is what explains which fields appear.
    edit(building);

    expect(screen.getByText("bld-1")).toBeInTheDocument();
    expect(screen.getByText("Polygon")).toBeInTheDocument();
  });

  it("closes without touching the model", () => {
    const onClose = vi.fn();
    useModelStore.getState().addFeature(building);
    render(<FeatureEditor featureId="bld-1" onClose={onClose} />);

    fireEvent.click(
      screen.getByRole("button", { name: m.tooltip_close_editor() }),
    );

    expect(onClose).toHaveBeenCalled();
    expect(useModelStore.getState().features).toHaveLength(1);
  });

  it("prefers a feature over a receiver that shares its id", () => {
    // Ids are validated as unique across both collections, but the editor
    // resolves them one after the other and only the feature branch can
    // delete. A collision must not land on the half that cannot.
    useModelStore.getState().addFeature({ ...building, id: "shared" });
    useModelStore.getState().addReceiver({ ...receiver, id: "shared" });
    render(<FeatureEditor featureId="shared" onClose={vi.fn()} />);

    expect(screen.getByText("Polygon")).toBeInTheDocument();
    expect(screen.queryByText(m.label_receiver())).not.toBeInTheDocument();
  });
});

describe("FeatureEditor labels follow the locale", () => {
  it("names the kind in the active language", () => {
    // The regression the file documents: both label sets were resolved at
    // module scope, so they froze to whatever locale was active when this
    // module was first imported. Switching to German relabelled the rest of
    // the app and left this panel in English until a full reload. Calling
    // them at module scope again would return "Building" here.
    useLocale("de");
    edit(building);

    expect(screen.getByRole("heading", { level: 3 })).toHaveTextContent(
      "Gebäude",
    );
  });

  it("names the vehicle classes in the active language", () => {
    // The second frozen set, reached only through the RLS-19 fields.
    useLocale("de");
    edit(lineSource);

    expect(
      numberField("road-1", "speed_pkw_kph").labels?.[0],
    ).toHaveTextContent("Pkw");
    expect(
      numberField("road-1", "traffic_night_krad").labels?.[0],
    ).toHaveTextContent("Krad");
  });

  it("names a barrier apart from a building", () => {
    // The kind switch has three arms and only one is exercised by the rest of
    // this file; a copy-paste between them is silent.
    edit(barrier);
    expect(screen.getByRole("heading", { level: 3 })).toHaveTextContent(
      m.option_barrier(),
    );
  });
});

describe("FeatureEditor height field", () => {
  it("shows the stored height for a building", () => {
    edit(building);
    expect(screen.getByLabelText(m.label_height_m())).toHaveValue(12);
  });

  it("commits a new height on blur, not on every keystroke", () => {
    // Committing per keystroke would push one undo entry per digit, so "12"
    // would take two Ctrl+Z to undo and would pass through the height 1.
    edit(building);
    const input = screen.getByLabelText(m.label_height_m());

    fireEvent.change(input, { target: { value: "20" } });
    expect(useModelStore.getState().getFeatureById("bld-1")?.heightM).toBe(12);

    fireEvent.blur(input);
    expect(useModelStore.getState().getFeatureById("bld-1")?.heightM).toBe(20);
  });

  it("refuses a height of zero or less", () => {
    // The schema requires `height_m > 0`. Accepting 0 here would write a model
    // that `aconiq validate` rejects, with nothing on this panel to say so.
    edit(building);
    const input = screen.getByLabelText(m.label_height_m());

    type(input, "0");
    expect(useModelStore.getState().getFeatureById("bld-1")?.heightM).toBe(12);

    type(input, "-5");
    expect(useModelStore.getState().getFeatureById("bld-1")?.heightM).toBe(12);
  });

  it("keeps the previous height when the field is emptied", () => {
    // A building without a height is invalid, so there is nothing to fall back
    // to — unlike the acoustics fields, where empty means "use the run
    // default".
    edit(building);

    type(screen.getByLabelText(m.label_height_m()), "");

    expect(useModelStore.getState().getFeatureById("bld-1")?.heightM).toBe(12);
  });

  it("edits a barrier's height through the same field", () => {
    edit(barrier);

    type(screen.getByLabelText(m.label_height_m()), "4.5");

    expect(useModelStore.getState().getFeatureById("bar-1")?.heightM).toBe(4.5);
  });
});

describe("FeatureEditor source type", () => {
  /** The derived value, which names itself after its own label. */
  function derivedSourceType(featureId: string): HTMLElement {
    const element = document.getElementById(`${featureId}-source-type`);
    if (!(element instanceof HTMLElement)) {
      throw new Error("no derived source type rendered");
    }
    return element;
  }

  it("reads the source type off the geometry rather than off a select", () => {
    // The select this replaces could be set to `area` on a LineString. The map
    // drew the result happily and `aconiq run` refused it with
    // `source.geometry.mismatch`, with nothing on the panel to say so.
    edit(lineSource);

    expect(
      screen.queryByRole("combobox", { name: m.label_source_type() }),
    ).toBeNull();
    expect(derivedSourceType("road-1")).toHaveTextContent(
      m.option_source_type_line(),
    );
    expect(screen.getByText(m.msg_source_type_derived())).toBeInTheDocument();
  });

  it("derives a point and an area from their geometries too", () => {
    // "Point" is also the geometry type one row up, which is why the value is
    // read through the id its label points at rather than by its text.
    edit(source);
    expect(derivedSourceType("src-1")).toHaveTextContent(
      m.option_source_type_point(),
    );

    useModelStore.getState().reset();
    edit({ ...building, id: "area-1", kind: "source", sourceType: "area" });
    expect(derivedSourceType("area-1")).toHaveTextContent(
      m.option_source_type_area(),
    );
  });

  it("offers to correct a declared type the geometry contradicts", () => {
    // Deriving alone would leave an imported contradiction unrepairable: the
    // property is still in the model and no control could rewrite it.
    edit({ ...lineSource, sourceType: "area" });

    fireEvent.click(
      screen.getByRole("button", {
        name: m.action_apply_derived_source_type(),
      }),
    );

    expect(useModelStore.getState().getFeatureById("road-1")?.sourceType).toBe(
      "line",
    );
  });

  it("offers no correction when the declaration already agrees", () => {
    edit(lineSource);
    expect(
      screen.queryByRole("button", {
        name: m.action_apply_derived_source_type(),
      }),
    ).toBeNull();
  });

  it("shows the RLS-19 fields only for a line source", () => {
    // RLS-19 road emission is a line-source model: speeds, traffic volumes and
    // a gradient mean nothing on a point or an area source, and offering them
    // there would write properties no standard reads.
    edit(source);
    expect(
      screen.queryByText(m.label_section_source_acoustics()),
    ).not.toBeInTheDocument();
  });

  it("shows the RLS-19 fields for a line source", () => {
    edit(lineSource);
    expect(
      screen.getByText(m.label_section_source_acoustics()),
    ).toBeInTheDocument();
  });
});

describe("FeatureEditor RLS-19 number fields", () => {
  it("shows a stored value", () => {
    edit({ ...lineSource, properties: { road_speed_kph: 50 } });
    expect(numberField("road-1", "road_speed_kph")).toHaveValue(50);
  });

  it("leaves the field empty when nothing is stored", () => {
    // Empty means "use the run default", and the placeholder says so. Showing
    // the standard's default here would make it look chosen for this road.
    edit(lineSource);
    const input = numberField("road-1", "road_speed_kph");

    expect(input).toHaveValue(null);
    expect(input).toHaveAttribute(
      "placeholder",
      m.placeholder_use_run_default(),
    );
  });

  it("commits on blur", () => {
    edit(lineSource);

    type(numberField("road-1", "traffic_day_pkw"), "800");

    expect(storedProperties("road-1")["traffic_day_pkw"]).toBe(800);
  });

  it("keeps a zero, which is not the same as unset", () => {
    // Zero night-time lorries is a real, assessable input; unset falls back to
    // the run default. A truthiness check instead of `Number.isFinite` would
    // collapse the two and quietly substitute the default.
    edit(lineSource);

    type(numberField("road-1", "traffic_night_lkw2"), "0");

    expect(storedProperties("road-1")["traffic_night_lkw2"]).toBe(0);
  });

  it("clears the property when the field is emptied", () => {
    // Not "writes 0" and not "writes undefined": the key has to leave the
    // model, because a property that is present with no value is what the
    // GeoJSON schema rejects.
    edit({ ...lineSource, properties: { road_speed_kph: 50 } });

    type(numberField("road-1", "road_speed_kph"), "");

    expect(storedProperties("road-1")).not.toHaveProperty("road_speed_kph");
  });

  it("drops the properties object entirely once the last key is cleared", () => {
    // `ModelFeature.properties` is "absent or a value"; an empty object
    // survives a round trip through the project file as `"properties": {}`,
    // which is noise in every diff of the model.
    edit({ ...lineSource, properties: { road_speed_kph: 50 } });

    type(numberField("road-1", "road_speed_kph"), "");

    expect(
      useModelStore.getState().getFeatureById("road-1"),
    ).not.toHaveProperty("properties");
  });

  it("never writes NaN, whatever is pasted into the field", () => {
    // A `type="number"` input reports an unparseable entry as the empty
    // string, so "fifty" and an overflowing "1e999" both arrive here as a
    // clear rather than as a number. What must not happen is the third
    // option: `Number.parseFloat` handed straight to the model, where NaN
    // serializes to `null` in the project file and reaches the engine as a
    // missing value it cannot name.
    for (const garbage of ["fifty", "1e999", "1.2.3"]) {
      edit({ ...lineSource, properties: { road_speed_kph: 50 } });
      type(numberField("road-1", "road_speed_kph"), garbage);

      const stored = storedProperties("road-1")["road_speed_kph"];
      expect(stored).not.toBeNaN();
      expect(stored).toBeUndefined();
      useModelStore.getState().reset();
    }
  });

  it("reads an imported value stored under its alias", () => {
    // SoundPLAN and the CSV importer write `road_gradient_percent`; the editor
    // canonicalises on `gradient_percent`. Reading only the canonical key
    // would show an imported road as having no gradient at all.
    edit({ ...lineSource, properties: { road_gradient_percent: -4 } });

    expect(numberField("road-1", "gradient_percent")).toHaveValue(-4);
  });

  it("replaces the alias rather than leaving two values behind", () => {
    // The real defect this guards: writing the canonical key while leaving the
    // alias in place puts two disagreeing gradients in the model, and which
    // one is used then depends on the reader.
    edit({ ...lineSource, properties: { road_gradient_percent: -4 } });

    type(numberField("road-1", "gradient_percent"), "6");

    expect(storedProperties("road-1")["gradient_percent"]).toBe(6);
    expect(storedProperties("road-1")).not.toHaveProperty(
      "road_gradient_percent",
    );
  });

  it("holds the gradient to the range RLS-19 tabulates", () => {
    // Beyond ±12% the standard has no correction to apply, so a value outside
    // it is not a steep road but a unit error.
    edit(lineSource);
    const input = numberField("road-1", "gradient_percent");

    expect(input).toHaveAttribute("min", "-12");
    expect(input).toHaveAttribute("max", "12");
  });

  it("allows a negative reflection surcharge but no negative traffic", () => {
    // The surcharge is a dB correction and may go either way; a vehicle count
    // cannot. The distinction is carried only by the `min` attributes.
    edit(lineSource);

    expect(
      numberField("road-1", "reflection_surcharge_db"),
    ).not.toHaveAttribute("min");
    expect(numberField("road-1", "traffic_day_pkw")).toHaveAttribute(
      "min",
      "0",
    );
    expect(numberField("road-1", "speed_pkw_kph")).toHaveAttribute(
      "min",
      "0.1",
    );
  });

  it("offers a speed and a traffic volume for all four vehicle classes", () => {
    // RLS-19 emits per class. A missing class is not a visible gap on the
    // panel — the grid just has one fewer cell — and its traffic silently
    // falls back to the run default.
    edit(lineSource);

    for (const suffix of ["pkw", "lkw1", "lkw2", "krad"]) {
      expect(numberField("road-1", `speed_${suffix}_kph`)).toBeInTheDocument();
      expect(
        numberField("road-1", `traffic_day_${suffix}`),
      ).toBeInTheDocument();
      expect(
        numberField("road-1", `traffic_night_${suffix}`),
      ).toBeInTheDocument();
    }
  });
});

describe("FeatureEditor RLS-19 provenance", () => {
  it("says a value was inferred on import", () => {
    // An inferred value is a guess the importer made from road class or
    // tagging, not something the user chose, and it is the reason the source
    // is flagged for review.
    edit({
      ...lineSource,
      properties: { road_speed_kph: 50, road_speed_kph_inferred: true },
    });

    expect(
      screen.getByText(m.msg_source_acoustics_inferred()),
    ).toBeInTheDocument();
  });

  it("stops calling a value inferred once it has been typed over", () => {
    // The user has now chosen it, so the review flag no longer applies to this
    // field. Leaving `_inferred` behind would keep asking about a value that
    // was checked.
    edit({
      ...lineSource,
      properties: { road_speed_kph: 50, road_speed_kph_inferred: true },
    });

    type(numberField("road-1", "road_speed_kph"), "70");

    expect(storedProperties("road-1")).not.toHaveProperty(
      "road_speed_kph_inferred",
    );
    expect(
      screen.queryByText(m.msg_source_acoustics_inferred()),
    ).not.toBeInTheDocument();
  });

  it("explains the empty state on a field that was never inferred", () => {
    edit(lineSource);
    expect(
      screen.getAllByText(m.msg_source_acoustics_default_fallback()).length,
    ).toBeGreaterThan(0);
  });

  it("warns when the source carries imported acoustics to review", () => {
    // The same flag the validator raises `source.rls19.review_required` on.
    // The panel is where it can actually be acted on.
    edit({
      ...lineSource,
      properties: { source_acoustics_review_required: true },
    });

    expect(
      screen.getByText(m.msg_source_acoustics_review_required()),
    ).toBeInTheDocument();
  });

  it("stays quiet on a source that was drawn rather than imported", () => {
    edit(lineSource);
    expect(
      screen.queryByText(m.msg_source_acoustics_review_required()),
    ).not.toBeInTheDocument();
  });

  it("lets the reader sign the imported acoustics off, and take it back", () => {
    // The warning asks for a decision the panel had no way to record, so the
    // finding survived every correct answer to it. This button is the answer.
    const imported = {
      ...lineSource,
      properties: { source_acoustics_review_required: true },
    };
    edit(imported);

    fireEvent.click(
      screen.getByRole("button", { name: m.action_mark_acoustics_reviewed() }),
    );

    expect(
      useModelStore.getState().getFeatureById(lineSource.id)?.properties,
    ).toEqual({
      source_acoustics_review_required: true,
      source_acoustics_reviewed: true,
    });
    expect(
      screen.queryByText(m.msg_source_acoustics_review_required()),
    ).not.toBeInTheDocument();
    expect(
      screen.getByText(m.msg_source_acoustics_reviewed()),
    ).toBeInTheDocument();

    // Still offered in reverse, so an accidental sign-off is visible and
    // reversible where it was made rather than only on the undo stack.
    fireEvent.click(
      screen.getByRole("button", {
        name: m.action_unmark_acoustics_reviewed(),
      }),
    );

    expect(
      useModelStore.getState().getFeatureById(lineSource.id)?.properties,
    ).toEqual({ source_acoustics_review_required: true });
    expect(
      screen.getByText(m.msg_source_acoustics_review_required()),
    ).toBeInTheDocument();
  });

  it("offers no sign-off on a source no import flagged", () => {
    // There is nothing to accept: the values are the reader's own.
    edit(lineSource);
    expect(
      screen.queryByRole("button", {
        name: m.action_mark_acoustics_reviewed(),
      }),
    ).not.toBeInTheDocument();
  });
});

describe("FeatureEditor RLS-19 select fields", () => {
  /**
   * Addressed through the label, which is what every select on the panel now
   * carries: they used to be picked out by their position among the
   * comboboxes, and a Radix trigger with no label reads out as its own current
   * value — so three of them in a row announced "SMA", "none", "none".
   */
  function acousticsSelect(which: "surface" | "junction"): HTMLElement {
    return selectField(
      "road-1",
      which === "surface" ? "surface_type" : "junction_type",
    );
  }

  it("offers every surface type RLS-19 tabulates, plus the run default", () => {
    // The list is the standard's, and `validate.ts` rejects anything outside
    // it — so a value missing here cannot be chosen, and one added here that
    // the validator does not know fails the model on the next run.
    edit(lineSource);
    fireEvent.click(acousticsSelect("surface"));

    expect(
      screen.getByRole("option", { name: m.option_use_run_default() }),
    ).toBeInTheDocument();
    for (const surface of RLS19_SURFACE_TYPES) {
      expect(screen.getByRole("option", { name: surface })).toBeInTheDocument();
    }
  });

  it("offers every junction type", () => {
    edit(lineSource);
    fireEvent.click(acousticsSelect("junction"));

    for (const junction of RLS19_JUNCTION_TYPES) {
      expect(
        screen.getByRole("option", { name: junction }),
      ).toBeInTheDocument();
    }
  });

  it("writes the chosen surface type", () => {
    edit(lineSource);

    fireEvent.click(acousticsSelect("surface"));
    fireEvent.click(screen.getByRole("option", { name: "OPA" }));

    expect(storedProperties("road-1")["surface_type"]).toBe("OPA");
  });

  it("reads an imported surface type stored under its alias", () => {
    edit({ ...lineSource, properties: { road_surface_type: "Beton" } });
    expect(acousticsSelect("surface")).toHaveTextContent("Beton");
  });

  it("replaces the surface alias rather than leaving both keys", () => {
    edit({ ...lineSource, properties: { road_surface_type: "Beton" } });

    fireEvent.click(acousticsSelect("surface"));
    fireEvent.click(screen.getByRole("option", { name: "SMA" }));

    expect(storedProperties("road-1")["surface_type"]).toBe("SMA");
    expect(storedProperties("road-1")).not.toHaveProperty("road_surface_type");
  });

  it("clears the property when the run default is chosen again", () => {
    // `__default__` is a sentinel for the select, not a value: writing it into
    // the model would fail the validator's surface-type check.
    edit({ ...lineSource, properties: { surface_type: "OPA" } });

    fireEvent.click(acousticsSelect("surface"));
    fireEvent.click(
      screen.getByRole("option", { name: m.option_use_run_default() }),
    );

    expect(storedProperties("road-1")).not.toHaveProperty("surface_type");
    expect(storedProperties("road-1")).not.toHaveProperty("__default__");
  });

  it("stops calling a surface type inferred once it has been chosen", () => {
    edit({
      ...lineSource,
      properties: { surface_type: "OPA", surface_type_inferred: true },
    });

    fireEvent.click(acousticsSelect("surface"));
    fireEvent.click(screen.getByRole("option", { name: "SMA" }));

    expect(storedProperties("road-1")).not.toHaveProperty(
      "surface_type_inferred",
    );
  });
});

describe("FeatureEditor receiver", () => {
  it("edits a receiver rather than a feature when the id is a receiver's", () => {
    useModelStore.getState().addReceiver(receiver);
    render(<FeatureEditor featureId="rcv-1" onClose={vi.fn()} />);

    expect(screen.getByRole("heading", { level: 3 })).toHaveTextContent(
      m.label_receiver(),
    );
    expect(screen.getByText("rcv-1")).toBeInTheDocument();
  });

  it("offers no source or acoustics fields on a receiver", () => {
    // A receiver is an immission point; it has a height and nothing else.
    useModelStore.getState().addReceiver(receiver);
    render(<FeatureEditor featureId="rcv-1" onClose={vi.fn()} />);

    expect(
      screen.queryByLabelText(m.label_source_type()),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByText(m.label_section_source_acoustics()),
    ).not.toBeInTheDocument();
  });

  it("commits a new receiver height on blur", () => {
    useModelStore.getState().addReceiver(receiver);
    render(<FeatureEditor featureId="rcv-1" onClose={vi.fn()} />);

    const input = screen.getByLabelText(m.label_height_m());
    fireEvent.change(input, { target: { value: "2.5" } });
    expect(useModelStore.getState().receivers[0]?.heightM).toBe(4);

    fireEvent.blur(input);
    expect(useModelStore.getState().receivers[0]?.heightM).toBe(2.5);
  });

  it("refuses a receiver height of zero or less", () => {
    // 16. BImSchV and TA Lärm assess at a stated height above ground; zero is
    // not a receiver, it is a missing value.
    useModelStore.getState().addReceiver(receiver);
    render(<FeatureEditor featureId="rcv-1" onClose={vi.fn()} />);

    type(screen.getByLabelText(m.label_height_m()), "0");

    expect(useModelStore.getState().receivers[0]?.heightM).toBe(4);
  });

  it("follows a height changed elsewhere in the model", () => {
    // The panel stays open while the same receiver is edited by an import or
    // an undo, so the field has to re-read rather than hold the value it was
    // mounted with.
    useModelStore.getState().addReceiver(receiver);
    render(<FeatureEditor featureId="rcv-1" onClose={vi.fn()} />);

    act(() => {
      useModelStore.getState().updateReceiver({ ...receiver, heightM: 9 });
    });

    expect(screen.getByLabelText(m.label_height_m())).toHaveValue(9);
  });
});

/**
 * The 16. BImSchV Gebietskategorie: the one receiver property the assessment
 * cannot proceed without. Until this select existed the panel offered height
 * alone, so a receiver drawn on the map could never be assessed at all — it
 * was skipped for a property no part of the UI could write.
 */
describe("FeatureEditor receiver area category", () => {
  function areaCategoryField(): HTMLElement {
    return selectField("rcv-1", PROP_BIMSCHV16_AREA_CATEGORY);
  }

  function storedReceiverProperties(): Record<string, unknown> {
    return useModelStore.getState().getReceiverById("rcv-1")?.properties ?? {};
  }

  it("offers the four categories ThresholdsForCategory knows, by their German names", () => {
    // The statutory category names, not translations of them: the thresholds
    // are declared for "Kern-, Dorf- oder Mischgebiet", and an invented
    // English category would name a row the law does not have.
    useModelStore.getState().addReceiver(receiver);
    render(<FeatureEditor featureId="rcv-1" onClose={vi.fn()} />);

    fireEvent.click(areaCategoryField());

    for (const category of BIMSCHV16_AREA_CATEGORIES) {
      expect(
        screen.getByRole("option", {
          name: bimschv16AreaCategoryLabel(category),
        }),
      ).toBeInTheDocument();
    }
  });

  it("says an absent category skips the receiver rather than defaulting it", () => {
    // There is no default category. `assessReceiverFeature` reports "missing
    // 16. BImSchV area category property" and the receiver lands in
    // `ExportEnvelope.Skipped`, so the helper must not promise a run default.
    useModelStore.getState().addReceiver(receiver);
    render(<FeatureEditor featureId="rcv-1" onClose={vi.fn()} />);

    expect(
      screen.getByText(m.msg_bimschv16_area_category_absent()),
    ).toBeInTheDocument();
    expect(
      screen.queryByText(m.msg_source_acoustics_default_fallback()),
    ).toBeNull();

    fireEvent.click(areaCategoryField());
    expect(
      screen.queryByRole("option", { name: m.option_use_run_default() }),
    ).toBeNull();
    expect(
      screen.getByRole("option", { name: m.option_not_set() }),
    ).toBeInTheDocument();
  });

  it("writes the canonical value the Go enum owns, not the German label", () => {
    // `ParseAreaCategory` takes both, but the enum value is what the type
    // carries — and what every other reader of the model compares against.
    useModelStore.getState().addReceiver(receiver);
    render(<FeatureEditor featureId="rcv-1" onClose={vi.fn()} />);

    fireEvent.click(areaCategoryField());
    fireEvent.click(
      screen.getByRole("option", {
        name: bimschv16AreaCategoryLabel("mixed"),
      }),
    );

    expect(storedReceiverProperties()).toEqual({
      bimschv16_area_category: "mixed",
    });
  });

  it("commits on change rather than on blur", () => {
    // A select has no blur the way a number field does; the choice is the
    // commit, as it is for every other vocabulary in this panel.
    useModelStore.getState().addReceiver(receiver);
    render(<FeatureEditor featureId="rcv-1" onClose={vi.fn()} />);

    fireEvent.click(areaCategoryField());
    fireEvent.click(
      screen.getByRole("option", {
        name: bimschv16AreaCategoryLabel("commercial"),
      }),
    );

    expect(storedReceiverProperties()["bimschv16_area_category"]).toBe(
      "commercial",
    );
  });

  it("removes the property again when the category is unset", () => {
    useModelStore.getState().addReceiver({
      ...receiver,
      properties: { bimschv16_area_category: "residential" },
    });
    render(<FeatureEditor featureId="rcv-1" onClose={vi.fn()} />);

    fireEvent.click(areaCategoryField());
    fireEvent.click(screen.getByRole("option", { name: m.option_not_set() }));

    expect(storedReceiverProperties()).toEqual({});
  });

  it("keeps the receiver's other properties and its height", () => {
    useModelStore.getState().addReceiver({
      ...receiver,
      properties: { name: "IO 1" },
    });
    render(<FeatureEditor featureId="rcv-1" onClose={vi.fn()} />);

    fireEvent.click(areaCategoryField());
    fireEvent.click(
      screen.getByRole("option", {
        name: bimschv16AreaCategoryLabel("residential"),
      }),
    );

    expect(storedReceiverProperties()).toEqual({
      name: "IO 1",
      bimschv16_area_category: "residential",
    });
    expect(useModelStore.getState().receivers[0]?.heightM).toBe(4);
  });

  it("goes through the command stack, so the write is undoable", () => {
    // `updateReceiver` is a full-object replace; a write that bypassed it
    // would leave an edit no undo could reach.
    useModelStore.getState().addReceiver(receiver);
    render(<FeatureEditor featureId="rcv-1" onClose={vi.fn()} />);

    fireEvent.click(areaCategoryField());
    fireEvent.click(
      screen.getByRole("option", { name: bimschv16AreaCategoryLabel("mixed") }),
    );
    act(() => {
      useModelStore.getState().undo();
    });

    expect(storedReceiverProperties()).toEqual({});
  });

  it("follows a category changed elsewhere in the model", () => {
    useModelStore.getState().addReceiver(receiver);
    render(<FeatureEditor featureId="rcv-1" onClose={vi.fn()} />);

    act(() => {
      useModelStore.getState().updateReceiver({
        ...receiver,
        properties: { bimschv16_area_category: "commercial" },
      });
    });

    expect(areaCategoryField()).toHaveTextContent(
      bimschv16AreaCategoryLabel("commercial"),
    );
  });

  it("shows an imported spelling the enum does not list rather than blanking it", () => {
    // `ParseAreaCategory` accepts "allgemeines Wohngebiet" on this very key,
    // so such a receiver is assessable — but the value matches none of the
    // four items, and a Radix select with no matching item falls back to its
    // placeholder. That would read as "not set" on a receiver that is set.
    useModelStore.getState().addReceiver({
      ...receiver,
      properties: { bimschv16_area_category: "allgemeines Wohngebiet" },
    });
    render(<FeatureEditor featureId="rcv-1" onClose={vi.fn()} />);

    expect(areaCategoryField()).toHaveTextContent("allgemeines Wohngebiet");
    expect(areaCategoryField()).not.toHaveTextContent(m.option_not_set());

    // And it stays a value that can be replaced or cleared.
    fireEvent.click(areaCategoryField());
    fireEvent.click(
      screen.getByRole("option", { name: bimschv16AreaCategoryLabel("mixed") }),
    );

    expect(storedReceiverProperties()).toEqual({
      bimschv16_area_category: "mixed",
    });
  });
});

/**
 * The panel as a dialog: it opens on a selection, it is the only thing that
 * can edit that selection, and it has to be leavable by keyboard. None of that
 * was true while it was a bare `MapPanel` — focus stayed wherever the click
 * left it, Tab walked out into the page behind the map, and Escape did
 * nothing.
 */
describe("FeatureEditor dialog behaviour", () => {
  it("is a dialog named by its own heading", () => {
    edit(building);

    const dialog = screen.getByRole("dialog", { name: m.option_building() });
    expect(dialog).toContainElement(screen.getByRole("heading", { level: 3 }));
  });

  it("takes focus when it opens, without landing in a field", () => {
    // Focusing the first input would turn the next keystroke into an edit of a
    // feature the user has only just clicked on.
    edit(building);

    expect(document.activeElement).toBe(screen.getByRole("dialog"));
  });

  it("closes on Escape and hands focus back to the content", () => {
    const onClose = vi.fn();
    useModelStore.getState().addFeature(building);
    render(
      <div id={MAIN_CONTENT_ID} tabIndex={-1}>
        <FeatureEditor featureId="bld-1" onClose={onClose} />
      </div>,
    );

    fireEvent.keyDown(screen.getByRole("dialog"), { key: "Escape" });

    expect(onClose).toHaveBeenCalled();
    expect(document.activeElement?.id).toBe(MAIN_CONTENT_ID);
  });

  it("wraps Tab and Shift+Tab inside the panel", () => {
    edit(building);
    const close = screen.getByRole("button", {
      name: m.tooltip_close_editor(),
    });
    const remove = deleteButton();

    remove.focus();
    fireEvent.keyDown(remove, { key: "Tab" });
    expect(document.activeElement).toBe(close);

    fireEvent.keyDown(close, { key: "Tab", shiftKey: true });
    expect(document.activeElement).toBe(remove);
  });

  it("leaves the delete confirmation its own Escape", () => {
    // React bubbles synthetic events through portals, so without the
    // `contains` guard an Escape meant for the confirmation would close the
    // panel underneath it as well.
    const onClose = vi.fn();
    useModelStore.getState().addFeature(building);
    render(<FeatureEditor featureId="bld-1" onClose={onClose} />);

    fireEvent.click(deleteButton());
    fireEvent.keyDown(screen.getByRole("alertdialog"), { key: "Escape" });

    expect(onClose).not.toHaveBeenCalled();
  });
});

/**
 * RLS-19 Nr. 3.4. `validate.ts` has refused a half-filled Parkplatz since it
 * learned the §3.4 rules, and until now no control on this panel could answer
 * the finding: the fields simply did not exist.
 */
describe("FeatureEditor Parkplatz fields", () => {
  const parking: ModelFeature = {
    id: "lot-1",
    kind: "source",
    sourceType: "area",
    geometry: {
      type: "Polygon",
      coordinates: [
        [
          [10, 51],
          [10.001, 51],
          [10.001, 51.001],
          [10, 51.001],
          [10, 51],
        ],
      ],
    },
  };

  it("appears on an area source and not on a line one", () => {
    edit(parking);
    expect(screen.getByText(m.label_section_parking())).toBeInTheDocument();

    useModelStore.getState().reset();
    edit(lineSource);
    expect(screen.queryByText(m.label_section_parking())).toBeNull();
  });

  it("writes the number of Stellplätze", () => {
    edit(parking);

    type(numberField("lot-1", "rls19_parking_num_spaces"), "40");

    expect(storedProperties("lot-1")["rls19_parking_num_spaces"]).toBe(40);
  });

  it("offers the Tabelle 6 rows and no run default to fall back on", () => {
    // An omitted Parkplatztyp is not Pkw — it selects the surcharge row, and
    // the rows differ by up to 10 dB. Offering "use run default" here would be
    // a control promising a default the standard does not have.
    edit(parking);
    fireEvent.click(selectField("lot-1", "rls19_parking_type"));

    for (const lotType of RLS19_PARKING_LOT_TYPES) {
      expect(screen.getByRole("option", { name: lotType })).toBeInTheDocument();
    }
    expect(
      screen.queryByRole("option", { name: m.option_use_run_default() }),
    ).toBeNull();
    expect(
      screen.getByRole("option", { name: m.option_not_set() }),
    ).toBeInTheDocument();
  });

  // Built from the renderer rather than typed out, so the assertion follows the
  // catalogue instead of pinning one locale's wording to this file.
  const parkingTypeMissing = (): string =>
    validationIssueText({
      level: "error",
      code: "source.rls19.parking.parking_type.missing",
      featureId: "lot-1",
      params: {
        field: PROP_PARKING_TYPE,
        expected: RLS19_PARKING_LOT_TYPES.join(", "),
      },
    });

  it("writes a Parkplatztyp and clears the finding that asked for it", () => {
    // One parking property present is what marks the feature as a Parkplatz at
    // all, so the rest become findings rather than silence.
    useModelStore.getState().addFeature({
      ...parking,
      properties: { rls19_parking_num_spaces: 40 },
    });
    render(<FeatureEditor featureId="lot-1" onClose={vi.fn()} />);

    expect(screen.getByText(parkingTypeMissing())).toBeInTheDocument();

    fireEvent.click(selectField("lot-1", "rls19_parking_type"));
    fireEvent.click(screen.getByRole("option", { name: "lkw-omnibus" }));

    expect(storedProperties("lot-1")["rls19_parking_type"]).toBe("lkw-omnibus");
    expect(screen.queryByText(parkingTypeMissing())).toBeNull();
  });

  it("does not promise a run default for the movement rates", () => {
    // Zero is the silence sentinel: a rate that quietly fell back to a default
    // would report an occupied Parkplatz as inaudible, or as busier than it is.
    edit(parking);
    const rate = numberField("lot-1", "rls19_parking_movements_per_space_day");

    expect(rate).not.toHaveAttribute("placeholder");
    expect(
      screen.getAllByText(m.msg_parking_movements_required()),
    ).toHaveLength(2);
  });

  it("offers the Tabelle 7 facility types that seed both rates", () => {
    edit(parking);
    fireEvent.click(selectField("lot-1", "rls19_parking_facility_type"));

    for (const facility of RLS19_PARKING_FACILITY_TYPES) {
      expect(
        screen.getByRole("option", { name: facility }),
      ).toBeInTheDocument();
    }
  });
});

/** Schall 03, the normative Anlage-2 property set. */
describe("FeatureEditor Schall 03 fields", () => {
  it("offers the track fields on a line source", () => {
    edit(lineSource);

    expect(screen.getByText(m.label_section_rail())).toBeInTheDocument();
    expect(
      numberField("road-1", "schall03_strecke_max_kph"),
    ).toBeInTheDocument();
  });

  it("writes a Fahrbahnart from the Tabelle 7 vocabulary", () => {
    edit(lineSource);

    fireEvent.click(selectField("road-1", "schall03_fahrbahn"));
    fireEvent.click(screen.getByRole("option", { name: "feste-fahrbahn" }));

    expect(storedProperties("road-1")["schall03_fahrbahn"]).toBe(
      "feste-fahrbahn",
    );
  });

  it("reports how many Zugarten the feature carries without editing them", () => {
    // A form for `schall03_operations` would be a second model editor, and
    // half of one is how an Fz composition silently loses a vehicle.
    edit({
      ...lineSource,
      properties: {
        schall03_operations: [
          { zugart: "ICE-3-Vollzug" },
          { zugart: "Gueterzug-E-Lok" },
        ],
      },
    });

    expect(screen.getByText(m.msg_rail_arrays_read_only())).toBeInTheDocument();
    expect(screen.getByText("2")).toBeInTheDocument();
  });

  it("removes a boolean's key instead of writing false", () => {
    // Every Schall 03 flag defaults to false, so absent and `false` mean the
    // same thing — and absent keeps a row of `false`s out of every model diff.
    edit(lineSource);

    fireEvent.click(screen.getByLabelText(m.label_rail_is_station()));
    expect(storedProperties("road-1")["schall03_is_station"]).toBe(true);

    fireEvent.click(screen.getByLabelText(m.label_rail_is_station()));
    expect(storedProperties("road-1")).not.toHaveProperty(
      "schall03_is_station",
    );
  });

  it("offers the barrier properties on a barrier", () => {
    edit(barrier);

    expect(
      screen.getByLabelText(m.label_rail_reflective()),
    ).toBeInTheDocument();
    expect(numberField("bar-1", "schall03_base_height_m")).toBeInTheDocument();
    expect(selectField("bar-1", "schall03_wall_surface")).toBeInTheDocument();
  });

  it("offers a building only the reflection opt-in and the wall surface", () => {
    // A building shields unconditionally — `height_m` is already required — so
    // there is nothing to switch on for that half.
    edit(building);

    expect(
      screen.getByLabelText(m.label_rail_reflecting_wall()),
    ).toBeInTheDocument();
    expect(selectField("bld-1", "schall03_wall_surface")).toBeInTheDocument();
    expect(screen.queryByLabelText(m.label_rail_reflective())).toBeNull();
  });
});

/**
 * A number field's own bounds, enforced where the model is written.
 *
 * `min`/`max` on an `<input type="number">` only mark it `:invalid`: constraint
 * validation gates a form submission and this panel submits nothing, so every
 * one of these values used to reach the model as typed and be refused by the Go
 * extractor at run time — with no frontend validator in between, because
 * `validate.ts` carries no Schall 03 rules.
 */
describe("FeatureEditor number field constraints", () => {
  it("refuses a Brückentyp outside the tabulated rows", () => {
    edit(lineSource);

    const input = numberField("road-1", "schall03_bridge_type");
    type(input, "5");

    expect(storedProperties("road-1")).not.toHaveProperty(
      "schall03_bridge_type",
    );
    expect(
      screen.getByText(m.msg_field_range({ min: 0, max: 4 })),
    ).toBeInTheDocument();
    expect(input).toHaveAttribute("aria-invalid", "true");
  });

  it("refuses a fractional Brückentyp, which names a row and not a quantity", () => {
    edit(lineSource);

    type(numberField("road-1", "schall03_bridge_type"), "1.5");

    expect(storedProperties("road-1")).not.toHaveProperty(
      "schall03_bridge_type",
    );
    expect(screen.getByText(m.msg_field_integer_only())).toBeInTheDocument();
  });

  it("refuses a water-body fraction above one", () => {
    edit(lineSource);

    type(numberField("road-1", "schall03_water_body_fraction"), "2");

    expect(storedProperties("road-1")).not.toHaveProperty(
      "schall03_water_body_fraction",
    );
    expect(
      screen.getByText(m.msg_field_range({ min: 0, max: 1 })),
    ).toBeInTheDocument();
  });

  it("takes a value its step would not land on, because step is not a bound", () => {
    // `schall03_strecke_max_kph` steps by 1 from a `min` of 0.1. Reading the
    // step as a constraint would refuse the smallest legal speed there is.
    edit(lineSource);

    type(numberField("road-1", "schall03_strecke_max_kph"), "0.1");

    expect(storedProperties("road-1")["schall03_strecke_max_kph"]).toBe(0.1);
  });

  it("reports a lower bound on its own without inventing an upper one", () => {
    edit(lineSource);

    type(numberField("road-1", "schall03_strecke_max_kph"), "0");

    expect(screen.getByText(m.msg_field_min({ min: 0.1 }))).toBeInTheDocument();
  });

  it("takes the refusal away once a legal value replaces it", () => {
    edit(lineSource);

    const input = numberField("road-1", "schall03_bridge_type");
    type(input, "5");
    type(input, "3");

    expect(storedProperties("road-1")["schall03_bridge_type"]).toBe(3);
    expect(
      screen.queryByText(m.msg_field_range({ min: 0, max: 4 })),
    ).toBeNull();
    expect(input).toHaveAttribute("aria-invalid", "false");
  });

  it("leaves a field with no bounds alone", () => {
    // `reflection_surcharge_db` is a correction that may go either way, so it
    // carries neither `min` nor `max` and nothing here may invent one.
    edit(lineSource);

    type(numberField("road-1", "reflection_surcharge_db"), "-2.5");

    expect(storedProperties("road-1")["reflection_surcharge_db"]).toBe(-2.5);
  });
});

/**
 * The validator's findings, on the panel that can act on them. The workspace's
 * validation panel offers a "go to" that opens this editor, and until now the
 * reader arrived with nothing repeating what the problem had been.
 */
describe("FeatureEditor inline issues", () => {
  const heightless: ModelFeature = {
    id: "bld-2",
    kind: "building",
    geometry: building.geometry,
  };

  it("shows the feature's own findings, code and all", () => {
    edit(heightless);

    expect(
      screen.getByText(m.msg_validation_building_height_required()),
    ).toBeInTheDocument();
    expect(screen.getByText("building.height.required")).toBeInTheDocument();
  });

  it("shows nothing for a feature the validator is happy with", () => {
    edit(building);

    expect(screen.queryByText("building.height.required")).toBeNull();
  });

  it("shows both findings where one code fires twice", () => {
    // A Parkplatz missing both movement rates pushes
    // `source.rls19.parking.movements.missing` once per period. Keyed on the
    // code alone React kept one of the two, so the panel asked for half of what
    // the run needs — and the reader who fixed the one line it showed came
    // straight back to the same refusal.
    edit({
      id: "lot-2",
      kind: "source",
      sourceType: "area",
      properties: { rls19_parking_num_spaces: 200, rls19_parking_type: "pkw" },
      geometry: {
        type: "Polygon",
        coordinates: [
          [
            [10, 51],
            [10.001, 51],
            [10.001, 51.001],
            [10, 51.001],
            [10, 51],
          ],
        ],
      },
    });

    expect(
      screen.getAllByText("source.rls19.parking.movements.missing"),
    ).toHaveLength(2);
  });

  it("shows no other feature's findings", () => {
    useModelStore.getState().addFeature(heightless);
    edit(building);

    expect(screen.queryByText("building.height.required")).toBeNull();
  });
});

describe("FeatureEditor link into the results", () => {
  /*
   * The other direction of the map→table move, for the population the map
   * click cannot serve.
   *
   * A click on a model receiver opens this editor rather than navigating —
   * that is the precedence rule, and it is the right one, because the editor
   * is the only thing that can change the receiver. So the way on to the row
   * has to be *in* the editor, and it is a real `<a>`: the same argument
   * `receiver-table.tsx` makes in the opposite direction. A button that
   * navigates has no href to copy, no middle-click, no context menu and no
   * entry in a screen reader's links rotor, and no axe rule catches the
   * substitution.
   *
   * Offered only for an id the run's own table holds. `useSelectableIds` is
   * the mirror of this on the results side, and the honesty rule is the same:
   * a link that navigates and then marks nothing is a promise the page cannot
   * keep.
   */

  const table: ReceiverTable = {
    indicator_order: ["Lden"],
    units: { Lden: "dB(A)" },
    records: [{ id: "rcv-1", x: 0, y: 0, height_m: 4, values: { Lden: 62.4 } }],
  };

  function completedRun(overrides: Partial<RunSummary> = {}): RunSummary {
    return {
      id: "run-7",
      scenario_id: "default",
      standard_id: "rls19-road",
      version: "2019",
      status: "completed",
      started_at: "2026-01-01T10:00:00Z",
      finished_at: "2026-01-01T10:00:05Z",
      log_path: "runs/run-7/run.log",
      artifacts: [
        {
          id: "art-receivers",
          kind: "run.result.receiver_table_json",
          path: "runs/run-7/results/receivers.json",
          created_at: "2026-01-01T10:00:05Z",
        },
      ],
      ...overrides,
    };
  }

  function editReceiver(id: string, run: RunSummary | null) {
    render(
      <MemoryRouter>
        <FeatureEditor featureId={id} resultRun={run} onClose={vi.fn()} />
      </MemoryRouter>,
    );
  }

  function resultsLink(id: string): HTMLElement | null {
    return screen.queryByRole("link", {
      name: m.action_show_receiver_in_results({ id }),
    });
  }

  beforeEach(() => {
    api.table = table;
    api.askedFor = [];
    useModelStore.getState().addReceiver(receiver);
  });

  it("offers a link to the row for a receiver the run computed", () => {
    editReceiver("rcv-1", completedRun());

    const link = resultsLink("rcv-1");
    expect(link).toHaveAttribute("href", "/results/run-7?receiver=rcv-1");
  });

  it("offers nothing when the run's table does not hold the receiver", () => {
    // A receiver added after the run, or one outside its calculation area:
    // the row is not there to scroll to, so there is nothing to link to.
    api.table = { ...table, records: [] };
    editReceiver("rcv-1", completedRun());

    expect(resultsLink("rcv-1")).toBeNull();
  });

  it("offers nothing, and asks for nothing, while no run is drawn", () => {
    // Nothing to link *to*. The hook must not even be called with an id: a
    // table fetched for a page with no results on it is a quarter of a
    // million rows nobody asked for.
    editReceiver("rcv-1", null);

    expect(resultsLink("rcv-1")).toBeNull();
    expect(api.askedFor).toEqual([]);
  });

  it("offers nothing for a run that wrote no receiver table", () => {
    editReceiver("rcv-1", completedRun({ artifacts: [] }));

    expect(resultsLink("rcv-1")).toBeNull();
    expect(api.askedFor).toEqual([null]);
  });

  it("escapes an id that would otherwise change the query", () => {
    const odd = { ...receiver, id: "R&1 #2" };
    useModelStore.getState().reset();
    useModelStore.getState().addReceiver(odd);
    api.table = {
      ...table,
      records: [{ id: "R&1 #2", x: 0, y: 0, height_m: 4, values: {} }],
    };
    editReceiver("R&1 #2", completedRun());

    expect(resultsLink("R&1 #2")).toHaveAttribute(
      "href",
      "/results/run-7?receiver=R%261+%232",
    );
  });

  it("offers nothing on a feature that is not a receiver", () => {
    // The run's table is receivers only; a source has no row in it.
    useModelStore.getState().addFeature(source);
    editReceiver("src-1", completedRun());

    expect(screen.queryByRole("link")).toBeNull();
  });
});
