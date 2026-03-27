package road

import "testing"

func TestDTVToHourly_Table2Values(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		dtv       float64
		roadClass Table2RoadClass
		period    Table2Period
		wantM     float64
		wantP1    float64
		wantP2    float64
	}{
		{name: "motorway day", dtv: 10000, roadClass: Table2RoadClassMotorwayExpressway, period: Table2PeriodDay, wantM: 555, wantP1: 3, wantP2: 11},
		{name: "motorway night", dtv: 10000, roadClass: Table2RoadClassMotorwayExpressway, period: Table2PeriodNight, wantM: 140, wantP1: 10, wantP2: 25},
		{name: "federal day", dtv: 10000, roadClass: Table2RoadClassFederalRoad, period: Table2PeriodDay, wantM: 575, wantP1: 3, wantP2: 7},
		{name: "federal night", dtv: 10000, roadClass: Table2RoadClassFederalRoad, period: Table2PeriodNight, wantM: 100, wantP1: 7, wantP2: 13},
		{name: "state county link day", dtv: 10000, roadClass: Table2RoadClassStateCountyMunicipalLinkRoad, period: Table2PeriodDay, wantM: 575, wantP1: 3, wantP2: 5},
		{name: "state county link night", dtv: 10000, roadClass: Table2RoadClassStateCountyMunicipalLinkRoad, period: Table2PeriodNight, wantM: 100, wantP1: 5, wantP2: 6},
		{name: "municipal day", dtv: 10000, roadClass: Table2RoadClassMunicipalRoad, period: Table2PeriodDay, wantM: 575, wantP1: 3, wantP2: 4},
		{name: "municipal night", dtv: 10000, roadClass: Table2RoadClassMunicipalRoad, period: Table2PeriodNight, wantM: 100, wantP1: 3, wantP2: 4},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := DTVToHourly(tt.dtv, tt.roadClass, tt.period)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !almostEqual(got.MPerHour, tt.wantM, 0.000001) {
				t.Fatalf("MPerHour: want %.6f, got %.6f", tt.wantM, got.MPerHour)
			}

			if !almostEqual(got.P1Percent, tt.wantP1, 0.000001) {
				t.Fatalf("P1Percent: want %.6f, got %.6f", tt.wantP1, got.P1Percent)
			}

			if !almostEqual(got.P2Percent, tt.wantP2, 0.000001) {
				t.Fatalf("P2Percent: want %.6f, got %.6f", tt.wantP2, got.P2Percent)
			}
		})
	}
}

func TestTable2HourlyTraffic_ToTrafficInput(t *testing.T) {
	t.Parallel()

	hourly, err := DTVToHourly(10000, Table2RoadClassFederalRoad, Table2PeriodDay)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	traffic := hourly.ToTrafficInput(5)
	if !almostEqual(traffic.PkwPerHour, 517.5, 0.000001) {
		t.Fatalf("PkwPerHour: want 517.500000, got %.6f", traffic.PkwPerHour)
	}

	if !almostEqual(traffic.Lkw1PerHour, 17.25, 0.000001) {
		t.Fatalf("Lkw1PerHour: want 17.250000, got %.6f", traffic.Lkw1PerHour)
	}

	if !almostEqual(traffic.Lkw2PerHour, 40.25, 0.000001) {
		t.Fatalf("Lkw2PerHour: want 40.250000, got %.6f", traffic.Lkw2PerHour)
	}

	if !almostEqual(traffic.KradPerHour, 5, 0.000001) {
		t.Fatalf("KradPerHour: want 5.000000, got %.6f", traffic.KradPerHour)
	}

	if !almostEqual(traffic.TotalPerHour(), 580, 0.000001) {
		t.Fatalf("TotalPerHour: want 580.000000, got %.6f", traffic.TotalPerHour())
	}
}

func TestDTVToHourly_InvalidInput(t *testing.T) {
	t.Parallel()

	if _, err := DTVToHourly(-1, Table2RoadClassFederalRoad, Table2PeriodDay); err == nil {
		t.Fatal("expected error for negative dtv")
	}

	if _, err := DTVToHourly(10000, Table2RoadClass(99), Table2PeriodDay); err == nil {
		t.Fatal("expected error for invalid road class")
	}

	if _, err := DTVToHourly(10000, Table2RoadClassFederalRoad, Table2Period(99)); err == nil {
		t.Fatal("expected error for invalid period")
	}
}
