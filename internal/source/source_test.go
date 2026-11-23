package source_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mocoarow/repohealth/internal/config"
	"github.com/mocoarow/repohealth/internal/source"
)

func testConfig() config.Config {
	return config.Config{
		Root:           "/repo",
		Version:        1,
		Areas:          []config.Area{{Name: "svc", Paths: []string{"svc"}}},
		Groups:         nil,
		Exclude:        []string{"svc/gen/**"},
		StablePackages: nil,
	}
}

func Test_Classify_shouldReturnKind_whenFileIsMeasured(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		file string
		want source.Kind
	}{
		{name: "go package file", file: "svc/domain/user.go", want: source.Kind{Lang: source.Go, Test: false}},
		{name: "go file at area root", file: "svc/main.go", want: source.Kind{Lang: source.Go, Test: false}},
		{name: "go test", file: "svc/domain/user_test.go", want: source.Kind{Lang: source.Go, Test: true}},
		{name: "typescript", file: "svc/app/a.tsx", want: source.Kind{Lang: source.Script, Test: false}},
		{name: "javascript module", file: "svc/app/a.mjs", want: source.Kind{Lang: source.Script, Test: false}},
		{name: "typescript test", file: "svc/app/a.test.ts", want: source.Kind{Lang: source.Script, Test: true}},
		{name: "typescript spec", file: "svc/app/a.spec.tsx", want: source.Kind{Lang: source.Script, Test: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// given
			cfg := testConfig()

			// when
			got, _ := source.Classify(cfg, tt.file)

			// then
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_Classify_shouldReportFalse_whenNoMeasurementReadsFile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		file string
	}{
		{name: "other extension", file: "svc/domain/user.md"},
		{name: "declaration file", file: "svc/app/types.d.ts"},
		{name: "excluded", file: "svc/gen/api.go"},
		{name: "outside every area", file: "tools/main.go"},
		{name: "go testdata", file: "svc/domain/testdata/broken.go"},
		{name: "go dot directory", file: "svc/.cache/x.go"},
		{name: "go underscore directory", file: "svc/_old/x.go"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// given
			cfg := testConfig()

			// when
			_, ok := source.Classify(cfg, tt.file)

			// then
			assert.False(t, ok)
		})
	}
}

func Test_GoProduction_shouldReportTrue_whenFileIsNonTestGo(t *testing.T) {
	t.Parallel()

	// given
	cfg := testConfig()

	// when
	got := source.GoProduction(cfg, "svc/domain/user.go")

	// then
	assert.True(t, got)
}

func Test_GoProduction_shouldReportFalse_whenFileIsNotNonTestGo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		file string
	}{
		{name: "go test", file: "svc/domain/user_test.go"},
		{name: "script", file: "svc/app/a.ts"},
		{name: "go testdata", file: "svc/domain/testdata/broken.go"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// given
			cfg := testConfig()

			// when
			got := source.GoProduction(cfg, tt.file)

			// then
			assert.False(t, got)
		})
	}
}

func Test_ScriptProduction_shouldReportTrue_whenFileIsNonTestScript(t *testing.T) {
	t.Parallel()

	// given
	cfg := testConfig()

	// when
	got := source.ScriptProduction(cfg, "svc/app/a.tsx")

	// then
	assert.True(t, got)
}

func Test_ScriptProduction_shouldReportFalse_whenFileIsNotNonTestScript(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		file string
	}{
		{name: "script test", file: "svc/app/a.test.ts"},
		{name: "go", file: "svc/domain/user.go"},
		{name: "declaration file", file: "svc/app/types.d.ts"},
		{name: "outside every area", file: "tools/a.ts"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// given
			cfg := testConfig()

			// when
			got := source.ScriptProduction(cfg, tt.file)

			// then
			assert.False(t, got)
		})
	}
}

func Test_Lang_MarshalText_shouldReturnShortName_whenLangIsKnown(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		lang source.Lang
		want string
	}{
		{name: "go", lang: source.Go, want: "go"},
		{name: "script", lang: source.Script, want: "ts"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// when
			got, err := tt.lang.MarshalText()

			// then
			require.NoError(t, err)
			assert.Equal(t, tt.want, string(got))
		})
	}
}

func Test_Lang_MarshalText_shouldReturnErrUnknownLang_whenLangIsZero(t *testing.T) {
	t.Parallel()

	// given
	var lang source.Lang

	// when
	_, err := lang.MarshalText()

	// then
	require.ErrorIs(t, err, source.ErrUnknownLang)
}

func Test_Lang_UnmarshalText_shouldReturnLang_whenTextIsKnown(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		text string
		want source.Lang
	}{
		{name: "go", text: "go", want: source.Go},
		{name: "ts", text: "ts", want: source.Script},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// given
			var lang source.Lang

			// when
			err := lang.UnmarshalText([]byte(tt.text))

			// then
			require.NoError(t, err)
			assert.Equal(t, tt.want, lang)
		})
	}
}

func Test_Lang_UnmarshalText_shouldReturnErrUnknownLang_whenTextIsUnknown(t *testing.T) {
	t.Parallel()

	// given
	var lang source.Lang

	// when
	err := lang.UnmarshalText([]byte("rust"))

	// then
	require.ErrorIs(t, err, source.ErrUnknownLang)
}
