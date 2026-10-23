package windowimport

import (
	"strconv"
	"strings"
	"testing"
)

const sampleCSV = `id,building,part,ring,edge,u_m,sill_z_m,width_m,height_m,sensitive,review_status
bedroom,house,0,0,0,2,3.9,1.2,1.5,true,confirmed
stairs,Musterstraße 1,,,2,1,1.2,0.8,1.0,false,
`

const sampleJSON = `[
  {"id": "bedroom", "building": "house", "edge": 0, "u_m": 2, "sill_z_m": 3.9, "width_m": 1.2, "height_m": 1.5,
   "sensitive": true, "review_status": "confirmed",
   "evidence": [{"kind": "plan", "source_id": "ansicht-sued"}]},
  {"id": "stairs", "building": "Musterstraße 1", "edge": 2, "u_m": 1, "sill_z_m": 1.2, "width_m": 0.8, "height_m": 1.0}
]`

func TestReadBothFormatsAlike(t *testing.T) {
	t.Parallel()

	for format, input := range map[string]string{FormatCSV: sampleCSV, FormatJSON: sampleJSON} {
		rows, err := Read(strings.NewReader(input), format)
		if err != nil {
			t.Fatalf("%s: %v", format, err)
		}

		if len(rows) != 2 {
			t.Fatalf("%s: %d rows, want 2", format, len(rows))
		}

		bedroom, stairs := rows[0], rows[1]
		if bedroom.ID != "bedroom" || bedroom.Building != "house" || bedroom.Edge != 0 || bedroom.UM != 2 ||
			bedroom.SillZM != 3.9 || bedroom.WidthM != 1.2 || bedroom.HeightM != 1.5 || !bedroom.Sensitive ||
			bedroom.ReviewStatus != "confirmed" || bedroom.Row != 1 {
			t.Errorf("%s: bedroom = %+v", format, bedroom)
		}

		// Part and ring default to 0, sensitive to false and the review
		// status to none, which placing reads as proposed.
		if stairs.Building != "Musterstraße 1" || stairs.Part != 0 || stairs.Ring != 0 || stairs.Edge != 2 ||
			stairs.Sensitive || stairs.ReviewStatus != "" || stairs.Row != 2 {
			t.Errorf("%s: stairs = %+v", format, stairs)
		}
	}

	rows, err := Read(strings.NewReader(sampleJSON), FormatJSON)
	if err != nil || len(rows[0].Evidence) != 1 {
		t.Errorf("JSON evidence = %+v, %v", rows[0].Evidence, err)
	}
}

func TestReadRefuses(t *testing.T) {
	t.Parallel()

	header := "id,building,edge,u_m,sill_z_m,width_m,height_m\n"
	one := `"id":"w","building":"h","edge":0,"u_m":1,"sill_z_m":1,"width_m":1,"height_m":1`

	for name, tc := range map[string]struct{ format, input, want string }{
		"csv unknown column":   {FormatCSV, "id,building,edge,u_m,sill_z_m,width_m,height_m,colour\nw,h,0,1,1,1,1,red\n", "colour"},
		"csv missing column":   {FormatCSV, "id,building,edge,u_m,sill_z_m,width_m\nw,h,0,1,1,1\n", "height_m"},
		"csv evidence column":  {FormatCSV, header[:len(header)-1] + ",evidence\nw,h,0,1,1,1,1,x\n", "evidence"},
		"csv bad number":       {FormatCSV, header + "w,h,0,one,1,1,1\n", "u_m"},
		"csv fractional edge":  {FormatCSV, header + "w,h,0.5,1,1,1,1\n", "edge"},
		"csv negative part":    {FormatCSV, "id,building,part,edge,u_m,sill_z_m,width_m,height_m\nw,h,-1,0,1,1,1,1\n", "part"},
		"csv empty number":     {FormatCSV, header + "w,h,0,,1,1,1\n", "u_m"},
		"csv bad sensitive":    {FormatCSV, header[:len(header)-1] + ",sensitive\nw,h,0,1,1,1,1,maybe\n", "sensitive"},
		"csv no id":            {FormatCSV, header + ",h,0,1,1,1,1\n", "id"},
		"csv no building":      {FormatCSV, header + "w,,0,1,1,1,1\n", "building"},
		"csv twice":            {FormatCSV, header + "w,h,0,1,1,1,1\nw,h,1,1,1,1,1\n", `"w"`},
		"csv no rows":          {FormatCSV, header, "no window"},
		"json unknown field":   {FormatJSON, `[{` + one + `,"colour":"red"}]`, "colour"},
		"json missing edge":    {FormatJSON, `[{"id":"w","building":"h","u_m":1,"sill_z_m":1,"width_m":1,"height_m":1}]`, "edge"},
		"json missing width":   {FormatJSON, `[{"id":"w","building":"h","edge":0,"u_m":1,"sill_z_m":1,"height_m":1}]`, "width_m"},
		"json not a list":      {FormatJSON, `{"id":"w"}`, "array"},
		"json trailing data":   {FormatJSON, `[{` + one + `}] []`, "after"},
		"json no rows":         {FormatJSON, `[]`, "no window"},
		"json evidence string": {FormatJSON, `[{` + one + `,"evidence":["x"]}]`, "evidence"},
		"unknown format":       {"xlsx", sampleCSV, "xlsx"},
	} {
		_, err := Read(strings.NewReader(tc.input), tc.format)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want it to mention %q", name, err, tc.want)
		}
	}
}

func TestFormatOf(t *testing.T) {
	t.Parallel()

	for path, want := range map[string]string{"w.csv": FormatCSV, "W.JSON": FormatJSON, "w.txt": ""} {
		if got := FormatOf(path); got != want {
			t.Errorf("FormatOf(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestReadRefusesTooManyRows(t *testing.T) {
	t.Parallel()

	var b strings.Builder

	b.WriteString("id,building,edge,u_m,sill_z_m,width_m,height_m\n")

	for i := range maxRows + 1 {
		b.WriteString("w" + strconv.Itoa(i) + ",h,0,1,1,1,1\n")
	}

	if _, err := Read(strings.NewReader(b.String()), FormatCSV); err == nil || !strings.Contains(err.Error(), "windows") {
		t.Errorf("err = %v, want the row limit", err)
	}
}
