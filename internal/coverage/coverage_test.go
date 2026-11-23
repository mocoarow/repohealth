package coverage_test

import (
	"io/fs"
	"testing"
	"testing/fstest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mocoarow/repohealth/internal/config"
	"github.com/mocoarow/repohealth/internal/coverage"
)

var reportTime = time.Date(2026, 10, 31, 23, 57, 0, 0, time.UTC)

func report(lcov string) *fstest.MapFile {
	return &fstest.MapFile{Data: []byte(lcov), ModTime: reportTime}
}

func testConfig(reports ...config.Report) config.Config {
	return config.Config{
		Root:    "/repo",
		Version: 1,
		Areas: []config.Area{
			{Name: "auth", Paths: []string{"svc-auth"}},
			{Name: "web", Paths: []string{"web"}},
		},
		Groups:         nil,
		Exclude:        []string{"svc-auth/gen/**"},
		StablePackages: nil,
		Coverage:       reports,
	}
}

const backendLcov = `SF:svc-auth/domain/user.go
DA:1,3
DA:2,0
end_of_record
SF:svc-auth/domain/group.go
DA:1,1
end_of_record
SF:svc-auth/gateway/repo.go
DA:5,0
DA:6,0
end_of_record
SF:svc-auth/main.go
DA:1,1
end_of_record
`

func Test_Measure_shouldSumLinesPerAreaAndLayer_whenReportCoversFiles(t *testing.T) {
	t.Parallel()

	// given
	fsys := fstest.MapFS{"coverage.lcov": report(backendLcov)}
	cfg := testConfig(config.Report{File: "coverage.lcov", Root: ""})

	// when
	got, err := coverage.Measure(fsys, cfg)

	// then
	require.NoError(t, err)
	assert.Equal(t, []coverage.Layer{
		{Area: "auth", Layer: ".", Covered: 1, Lines: 1},
		{Area: "auth", Layer: "domain", Covered: 2, Lines: 3},
		{Area: "auth", Layer: "gateway", Covered: 0, Lines: 2},
	}, got.Layers)
}

func Test_Measure_shouldReturnReportModTime_whenReportIsRead(t *testing.T) {
	t.Parallel()

	// given
	fsys := fstest.MapFS{"coverage.lcov": report(backendLcov)}
	cfg := testConfig(config.Report{File: "coverage.lcov", Root: ""})

	// when
	got, err := coverage.Measure(fsys, cfg)

	// then
	require.NoError(t, err)
	assert.Equal(t, []coverage.Report{{File: "coverage.lcov", ModTime: reportTime}}, got.Reports)
}

func Test_Measure_shouldResolvePathsAgainstRoot_whenReportHasRoot(t *testing.T) {
	t.Parallel()

	// given
	fsys := fstest.MapFS{"web/coverage/lcov.info": report("SF:app/a.ts\nDA:1,1\nDA:2,1\nend_of_record\n")}
	cfg := testConfig(config.Report{File: "web/coverage/lcov.info", Root: "web"})

	// when
	got, err := coverage.Measure(fsys, cfg)

	// then
	require.NoError(t, err)
	assert.Equal(t, []coverage.Layer{{Area: "web", Layer: "app", Covered: 2, Lines: 2}}, got.Layers)
}

func Test_Measure_shouldCountLineOnce_whenReportsOverlap(t *testing.T) {
	t.Parallel()

	// given
	fsys := fstest.MapFS{
		"a.lcov": report("SF:svc-auth/domain/user.go\nDA:1,4\nDA:2,0\nend_of_record\n"),
		"b.lcov": report("SF:svc-auth/domain/user.go\nDA:1,0\nend_of_record\n"),
	}
	cfg := testConfig(config.Report{File: "a.lcov", Root: ""}, config.Report{File: "b.lcov", Root: ""})

	// when
	got, err := coverage.Measure(fsys, cfg)

	// then
	require.NoError(t, err)
	assert.Equal(t, []coverage.Layer{{Area: "auth", Layer: "domain", Covered: 1, Lines: 2}}, got.Layers)
}

func Test_Measure_shouldIgnoreOtherRecords_whenReportHasThem(t *testing.T) {
	t.Parallel()

	// given
	lcov := "TN:\nSF:svc-auth/domain/user.go\nFN:1,f\nFNDA:1,f\nFNF:1\nFNH:1\n\n" +
		"DA:1,3,abc\nDA:2,0\nBRDA:1,0,0,1\nBRF:1\nBRH:1\nLF:2\nLH:1\nend_of_record\n"
	fsys := fstest.MapFS{"c.lcov": report(lcov)}
	cfg := testConfig(config.Report{File: "c.lcov", Root: ""})

	// when
	got, err := coverage.Measure(fsys, cfg)

	// then
	require.NoError(t, err)
	assert.Equal(t, []coverage.Layer{{Area: "auth", Layer: "domain", Covered: 1, Lines: 2}}, got.Layers)
}

func Test_Measure_shouldReadReport_whenLinesEndWithCRLF(t *testing.T) {
	t.Parallel()

	// given
	fsys := fstest.MapFS{"c.lcov": report("SF:svc-auth/domain/user.go\r\nDA:1,3\r\nDA:2,0\r\nend_of_record\r\n")}
	cfg := testConfig(config.Report{File: "c.lcov", Root: ""})

	// when
	got, err := coverage.Measure(fsys, cfg)

	// then
	require.NoError(t, err)
	assert.Equal(t, []coverage.Layer{{Area: "auth", Layer: "domain", Covered: 1, Lines: 2}}, got.Layers)
}

func Test_Measure_shouldLeaveOutLayer_whenItsFilesHaveNoLines(t *testing.T) {
	t.Parallel()

	// given
	fsys := fstest.MapFS{"c.lcov": report("SF:svc-auth/config/config.go\nLF:0\nend_of_record\n")}
	cfg := testConfig(config.Report{File: "c.lcov", Root: ""})

	// when
	got, err := coverage.Measure(fsys, cfg)

	// then
	require.NoError(t, err)
	assert.Empty(t, got.Layers)
}

func Test_Measure_shouldUseRepositoryRelativePath_whenSourcePathIsAbsoluteInsideRepository(t *testing.T) {
	t.Parallel()

	// given
	fsys := fstest.MapFS{"web/coverage/lcov.info": report("SF:/repo/web/app/a.ts\nDA:1,1\nend_of_record\n")}
	cfg := testConfig(config.Report{File: "web/coverage/lcov.info", Root: "web"})

	// when
	got, err := coverage.Measure(fsys, cfg)

	// then
	require.NoError(t, err)
	assert.Equal(t, []coverage.Layer{{Area: "web", Layer: "app", Covered: 1, Lines: 1}}, got.Layers)
}

func Test_Measure_shouldSkipFile_whenPathLeavesRepositoryUnderRootArea(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		file string
	}{
		{name: "relative path", file: "../other/x.go"},
		{name: "absolute path", file: "/elsewhere/x.go"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// given
			fsys := fstest.MapFS{"c.lcov": report("SF:" + tt.file + "\nDA:1,1\nend_of_record\n")}
			cfg := testConfig(config.Report{File: "c.lcov", Root: ""})
			cfg.Areas = []config.Area{{Name: "all", Paths: []string{"."}}}

			// when
			got, err := coverage.Measure(fsys, cfg)

			// then
			require.NoError(t, err)
			assert.Empty(t, got.Layers)
		})
	}
}

func Test_Measure_shouldSkipFile_whenFileIsNotInAnyArea(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		file string
		root string
	}{
		{name: "outside every area", file: "tools/t.go", root: ""},
		{name: "excluded", file: "svc-auth/gen/api.go", root: ""},
		{name: "absolute path outside repository", file: "/elsewhere/x.go", root: ""},
		{name: "absolute path outside repository with root", file: "/elsewhere/app/a.ts", root: "web"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// given
			fsys := fstest.MapFS{"c.lcov": report("SF:" + tt.file + "\nDA:1,1\nend_of_record\n")}
			cfg := testConfig(config.Report{File: "c.lcov", Root: tt.root})

			// when
			got, err := coverage.Measure(fsys, cfg)

			// then
			require.NoError(t, err)
			assert.Empty(t, got.Layers)
		})
	}
}

func Test_Measure_shouldCountSkippedRecords_whenSourceIsOutsideConfigDirectory(t *testing.T) {
	t.Parallel()

	// given
	lcov := "SF:/elsewhere/x.go\nDA:1,1\nend_of_record\nSF:../other/y.go\nDA:1,1\nend_of_record\n"
	fsys := fstest.MapFS{"c.lcov": report(lcov)}
	cfg := testConfig(config.Report{File: "c.lcov", Root: ""})

	// when
	got, err := coverage.Measure(fsys, cfg)

	// then
	require.NoError(t, err)
	assert.Equal(t, 2, got.Reports[0].Skipped)
}

func Test_Measure_shouldNotCountSkippedRecords_whenSourceIsInsideRepositoryButOutsideAreas(t *testing.T) {
	t.Parallel()

	// given
	fsys := fstest.MapFS{"c.lcov": report("SF:tools/t.go\nDA:1,1\nend_of_record\n")}
	cfg := testConfig(config.Report{File: "c.lcov", Root: ""})

	// when
	got, err := coverage.Measure(fsys, cfg)

	// then
	require.NoError(t, err)
	assert.Equal(t, 0, got.Reports[0].Skipped)
}

func Test_Measure_shouldReturnErrNotExist_whenReportIsMissing(t *testing.T) {
	t.Parallel()

	// given
	cfg := testConfig(config.Report{File: "coverage.lcov", Root: ""})

	// when
	_, err := coverage.Measure(fstest.MapFS{}, cfg)

	// then
	require.ErrorIs(t, err, fs.ErrNotExist)
}

func Test_Measure_shouldReturnErrMalformedReport_whenLineIsMalformed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		lcov string
	}{
		{name: "line before source file", lcov: "DA:1,1\n"},
		{name: "line after end of record", lcov: "SF:svc-auth/a.go\nend_of_record\nDA:1,1\n"},
		{name: "missing hits", lcov: "SF:svc-auth/a.go\nDA:1\n"},
		{name: "line not a number", lcov: "SF:svc-auth/a.go\nDA:x,1\n"},
		{name: "hits not a number", lcov: "SF:svc-auth/a.go\nDA:1,x\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// given
			fsys := fstest.MapFS{"c.lcov": report(tt.lcov)}
			cfg := testConfig(config.Report{File: "c.lcov", Root: ""})

			// when
			_, err := coverage.Measure(fsys, cfg)

			// then
			require.ErrorIs(t, err, coverage.ErrMalformedReport)
		})
	}
}
