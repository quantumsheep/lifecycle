package platform_test

import (
	"testing"

	"github.com/apex/log"
	"github.com/apex/log/handlers/memory"
	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"

	"github.com/buildpacks/lifecycle"
	"github.com/buildpacks/lifecycle/platform"
	h "github.com/buildpacks/lifecycle/testhelpers"
)

func TestExperimentalFeaturesGuard(t *testing.T) {
	spec.Run(t, "FeaturesGuard", testExperimentalFeaturesGuard, spec.Sequential(), spec.Report(report.Terminal{}))
}

func testExperimentalFeaturesGuard(t *testing.T, when spec.G, it spec.S) {
	var (
		logger     lifecycle.Logger
		logHandler *memory.Handler
	)

	it.Before(func() {
		logHandler = memory.New()
		logger = &log.Logger{Handler: logHandler}
	})

	when(".GuardExperimental", func() {
		when("CNB_PLATFORM_EXPERIMENTAL_MODE=warn", func() {
			it("warns", func() {
				platform.ExperimentalModeFeatures = "warn"
				h.AssertNil(t, platform.GuardExperimental("some-feature", logger))
				h.AssertEq(t, len(logHandler.Entries), 1)
				h.AssertEq(t, logHandler.Entries[0].Level, log.WarnLevel)
				h.AssertEq(t, logHandler.Entries[0].Message, "Platform requested experimental feature 'some-feature'")
			})
		})

		when("CNB_PLATFORM_EXPERIMENTAL_MODE=quiet", func() {
			it("succeeds silently", func() {
				platform.ExperimentalModeFeatures = "quiet"
				h.AssertNil(t, platform.GuardExperimental("some-feature", logger))
				h.AssertEq(t, len(logHandler.Entries), 0)
			})
		})

		when("CNB_PLATFORM_EXPERIMENTAL_MODE=error", func() {
			it("errors", func() {
				platform.ExperimentalModeFeatures = "error"
				err := platform.GuardExperimental("some-feature", logger)
				h.AssertEq(t, len(logHandler.Entries), 1)
				h.AssertEq(t, logHandler.Entries[0].Level, log.ErrorLevel)
				h.AssertEq(t, logHandler.Entries[0].Message, "Platform requested experimental feature 'some-feature'")
				h.AssertEq(t, err.Error(), "Experimental features are disabled by CNB_PLATFORM_EXPERIMENTAL_MODE=error")
			})
		})
	})
}
