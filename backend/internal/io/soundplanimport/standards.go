package soundplanimport

import (
	"sort"

	"github.com/aconiq/backend/internal/domain/project"
	"github.com/aconiq/backend/internal/standards/iso9613"
	rls19road "github.com/aconiq/backend/internal/standards/rls19/road"
	"github.com/aconiq/backend/internal/standards/schall03"
)

// StandardMapping describes how one SoundPlan standard ID maps into Aconiq.
type StandardMapping struct {
	SoundPlanID int
	Aconiq      project.StandardRef
	Supported   bool
	Warning     string
}

// MapStandardID resolves a SoundPlan standard ID to the closest Aconiq module.
func MapStandardID(id int) StandardMapping {
	switch id {
	case 20490:
		descriptor := schall03.Descriptor()
		return StandardMapping{
			SoundPlanID: id,
			Aconiq: project.StandardRef{
				Context: descriptor.Context,
				ID:      descriptor.ID,
				Version: descriptor.DefaultVersion,
				Profile: descriptor.Versions[0].DefaultProfile,
			},
			Supported: true,
		}
	case 10490:
		descriptor := rls19road.Descriptor()
		return StandardMapping{
			SoundPlanID: id,
			Aconiq: project.StandardRef{
				Context: descriptor.Context,
				ID:      descriptor.ID,
				Version: descriptor.DefaultVersion,
				Profile: descriptor.Versions[0].DefaultProfile,
			},
			Supported: true,
		}
	case 30000:
		descriptor := iso9613.Descriptor()
		return StandardMapping{
			SoundPlanID: id,
			Aconiq: project.StandardRef{
				Context: descriptor.Context,
				ID:      descriptor.ID,
				Version: descriptor.DefaultVersion,
				Profile: descriptor.Versions[0].DefaultProfile,
			},
			Supported: true,
		}
	default:
		return StandardMapping{
			SoundPlanID: id,
			Supported:   false,
			Warning:     "unsupported SoundPlan standard; import should continue with a warning",
		}
	}
}

// MapEnabledStandards returns deterministic mappings for all enabled SoundPlan standards.
func MapEnabledStandards(proj *Project) []StandardMapping {
	if proj == nil || len(proj.EnabledStandards) == 0 {
		return nil
	}

	ids := make([]int, 0, len(proj.EnabledStandards))
	for id, enabled := range proj.EnabledStandards {
		if enabled {
			ids = append(ids, id)
		}
	}

	sort.Ints(ids)

	mappings := make([]StandardMapping, 0, len(ids))
	for _, id := range ids {
		mappings = append(mappings, MapStandardID(id))
	}

	return mappings
}
