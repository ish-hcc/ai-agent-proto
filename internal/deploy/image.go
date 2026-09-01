package deploy

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/innogrid/ai-agent-proto/internal/model"
	"github.com/innogrid/ai-agent-proto/internal/tumblebug"
)

// imageCandidates bounds an image search. A handful is enough to prefer an
// accelerator image over a plain one without paging the whole catalog.
const imageCandidates = 20

// ImageChoice is the image selected for a spec, with why it was selected.
type ImageChoice struct {
	ID       string `json:"id"`
	OSType   string `json:"osType"`
	GPUImage bool   `json:"gpuImage"`
	// Reason explains the pick so an agent can report it and an operator can argue with it.
	Reason string `json:"reason"`
}

// ResolveImage picks a node image for an application on a given spec.
//
// An accelerator image is preferred when the application needs an accelerator:
// it ships the vendor driver, so the install commands do not have to. Falling
// back to a plain image is not a silent workaround, it is reported in Reason.
func (s *Service) ResolveImage(ctx context.Context, app *model.AppSpec, specID string) (*ImageChoice, error) {
	osType := app.Install.OSType
	if osType == "" {
		osType = s.defaultOSType
	}

	req := &tumblebug.SearchImageReq{
		MatchedSpecID: specID,
		OSType:        osType,
		MaxResults:    imageCandidates,
	}
	result, err := s.tumblebug.SearchImage(ctx, s.imageNamespace, req)
	if err != nil {
		return nil, err
	}
	if len(result.ImageList) == 0 {
		return nil, fmt.Errorf("no %s image is registered for spec %q", osType, specID)
	}

	needsAccelerator := app.Accelerator.Type != ""
	if needsAccelerator {
		for _, image := range result.ImageList {
			if image.IsGPUImage {
				log.Info().Str("appId", app.ID).Str("specId", specID).Str("imageId", image.ID).
					Bool("gpuImage", true).Msg("Resolved node image")
				return &ImageChoice{
					ID:       image.ID,
					OSType:   image.OSType,
					GPUImage: true,
					Reason:   "accelerator image: ships the vendor driver",
				}, nil
			}
		}
	}

	chosen := result.ImageList[0]
	reason := "first matching image for the spec"
	if needsAccelerator {
		reason = "no accelerator image is registered for this spec, so the driver must be installed by the application commands"
	}

	log.Info().Str("appId", app.ID).Str("specId", specID).Str("imageId", chosen.ID).
		Bool("gpuImage", false).Msg("Resolved node image")

	return &ImageChoice{
		ID:       chosen.ID,
		OSType:   chosen.OSType,
		GPUImage: false,
		Reason:   reason,
	}, nil
}
