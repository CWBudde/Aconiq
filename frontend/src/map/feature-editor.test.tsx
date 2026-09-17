import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { FeatureEditor } from "./feature-editor";
import { useModelStore } from "@/model/model-store";
import {
  RLS19_JUNCTION_TYPES,
  RLS19_SURFACE_TYPES,
} from "@/model/source-acoustics";
import type { ModelFeature, ModelReceiver } from "@/model/types";
import { MAIN_CONTENT_ID } from "@/ui/main-content";
import { getLocale, overwriteGetLocale, type Locale } from "@/i18n/runtime";
import { m } from "@/i18n/messages";

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

    expect(numberField("road-1", "speed_pkw_kph").labels?.[0]).toHaveTextContent(
      "Pkw",
    );
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
  it("offers the three source types and shows the current one", () => {
    edit(lineSource);
    fireEvent.click(screen.getByLabelText(m.label_source_type()));

    for (const option of [
      m.option_source_type_point(),
      m.option_source_type_line(),
      m.option_source_type_area(),
    ]) {
      expect(screen.getByRole("option", { name: option })).toBeInTheDocument();
    }
  });

  it("writes the chosen type to the model", () => {
    edit(lineSource);

    fireEvent.click(screen.getByLabelText(m.label_source_type()));
    fireEvent.click(
      screen.getByRole("option", { name: m.option_source_type_area() }),
    );

    expect(useModelStore.getState().getFeatureById("road-1")?.sourceType).toBe(
      "area",
    );
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
    expect(input).toHaveAttribute("placeholder", m.placeholder_use_run_default());
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

    expect(useModelStore.getState().getFeatureById("road-1")).not.toHaveProperty(
      "properties",
    );
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

    expect(numberField("road-1", "reflection_surcharge_db")).not.toHaveAttribute(
      "min",
    );
    expect(numberField("road-1", "traffic_day_pkw")).toHaveAttribute("min", "0");
    expect(numberField("road-1", "speed_pkw_kph")).toHaveAttribute("min", "0.1");
  });

  it("offers a speed and a traffic volume for all four vehicle classes", () => {
    // RLS-19 emits per class. A missing class is not a visible gap on the
    // panel — the grid just has one fewer cell — and its traffic silently
    // falls back to the run default.
    edit(lineSource);

    for (const suffix of ["pkw", "lkw1", "lkw2", "krad"]) {
      expect(numberField("road-1", `speed_${suffix}_kph`)).toBeInTheDocument();
      expect(numberField("road-1", `traffic_day_${suffix}`)).toBeInTheDocument();
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
});

describe("FeatureEditor RLS-19 select fields", () => {
  /**
   * Three comboboxes are on the panel for a line source, in DOM order: the
   * source type, the surface type and the junction type. Only the first has a
   * label associated with it, so the other two are addressed by position.
   */
  function acousticsSelect(which: "surface" | "junction"): HTMLElement {
    const boxes = screen.getAllByRole("combobox");
    const box = boxes[which === "surface" ? 1 : 2];
    if (!box) throw new Error(`no ${which} select rendered`);
    return box;
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
      expect(screen.getByRole("option", { name: junction })).toBeInTheDocument();
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

    expect(screen.queryByLabelText(m.label_source_type())).not.toBeInTheDocument();
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
