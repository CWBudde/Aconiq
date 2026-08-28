package schall03

import "github.com/aconiq/backend/internal/standards/framework"

// StandardData returns every coefficient source this module carries.
//
// It covers both chains deliberately. The digest answers "could these two runs
// have produced different numbers?", and a change to the preview data pack
// changes the module a normative run was produced by even though that run never
// read it. Erring towards a digest that moves too often is safe; one that stays
// still while coefficients move is not. Which chain a given run actually used
// is recorded separately, by ResolvedProvenanceMetadata.
func StandardData() framework.StandardData {
	return framework.StandardData{Tables: []framework.StandardDataTable{
		{Name: "anlage2/beiblatt1-fahrzeugkategorien", Value: FzKategorien},
		{Name: "anlage2/beiblatt1-kesselwagen", Value: KesselwagenTeilquellen},
		{Name: "anlage2/beiblatt1-zugarten", Value: Zugarten},
		{Name: "anlage2/beiblatt2-fahrzeugkategorien-strassenbahn", Value: FzKategorienStrassenbahn},
		{Name: "anlage2/beiblatt2-zugarten-strassenbahn", Value: ZugartStrassenbahn},
		{Name: "anlage2/beiblatt3-gleisbremsen", Value: gleisbremsTable},
		{Name: "anlage2/beiblatt3-rangierquellen", Value: beiblatt3YardSources()},
		{Name: "anlage2/oktavband-mittenfrequenzen", Value: BeiblattOctaveBandFrequencies},
		{Name: "anlage2/tabelle-06-geschwindigkeitsfaktor", Value: SpeedFactorBTable},
		{Name: "anlage2/tabelle-07-fahrbahnart", Value: C1FahrbahnartTable},
		{Name: "anlage2/tabelle-08-fahrbahnzustand", Value: C2SurfaceConditionTable},
		{Name: "anlage2/tabelle-09-bruecken", Value: BridgeCorrectionTable},
		{Name: "anlage2/tabelle-11-kurven", Value: CurveNoiseCorrectionTable},
		{Name: "anlage2/tabelle-15-fahrbahnart-strassenbahn", Value: C1StrassenbahnTable},
		{Name: "anlage2/tabelle-16-bruecken-strassenbahn", Value: BridgeCorrectionStrassenbahnTable},
		{Name: "anlage2/tabelle-17-luftabsorption", Value: AirAbsorptionAlpha},
		{Name: "preview/datapack", Value: BuiltinDataPack()},
	}}
}

// beiblatt3YardSources groups the Beiblatt 3 Rangierbahnhof sources that are
// declared as individual variables rather than as one table.
func beiblatt3YardSources() []YardSourceData {
	return []YardSourceData{
		Beiblatt3Kurvenfahrgeraeusch,
		Beiblatt3RetarderVerzoegerungsstrecke,
		Beiblatt3RetarderBeharrungsstreckeBase,
		Beiblatt3RetarderRangierenBase,
		Beiblatt3HemmschuhauflaufgeraeuschData,
		Beiblatt3AnreissenAbbremsenBase,
		Beiblatt3AuflaufstossByTech(false),
		Beiblatt3AuflaufstossByTech(true),
	}
}
