package unit_test

import (
	"flag"
	"github.com/ecopia-map/cesium_tiler/tools"
	"os"
	"strconv"
	"testing"
)

func TestInputFlagIsParsed(t *testing.T) {
	expected := "/home/user/file.las"
	os.Args = []string{"cesium_tiler", "-input=" + expected}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flags := tools.ParseFlags()
	if *flags.Input != expected {
		t.Errorf("Expected Input = %s, got %s", expected, *flags.Input)
	}
}

func TestIFlagIsParsed(t *testing.T) {
	expected := "/home/user/file.las"
	os.Args = []string{"cesium_tiler", "-i=" + expected}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flags := tools.ParseFlags()
	if *flags.Input != expected {
		t.Errorf("Expected Input = %s, got %s", expected, *flags.Input)
	}
}

func TestOutputFlagIsParsed(t *testing.T) {
	expected := "/home/user/output"
	os.Args = []string{"cesium_tiler", "-output=" + expected}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flags := tools.ParseFlags()
	if *flags.Output != expected {
		t.Errorf("Expected Output = %s, got %s", expected, *flags.Output)
	}
}

func TestOFlagIsParsed(t *testing.T) {
	expected := "/home/user/output"
	os.Args = []string{"cesium_tiler", "-o=" + expected}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flags := tools.ParseFlags()
	if *flags.Output != expected {
		t.Errorf("Expected Output = %s, got %s", expected, *flags.Output)
	}
}

func TestSridFlagIsParsed(t *testing.T) {
	expected := 32633
	os.Args = []string{"cesium_tiler", "-srid=" + strconv.Itoa(expected)}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flags := tools.ParseFlags()
	if *flags.Srid != expected {
		t.Errorf("Expected Srid = %d, got %d", expected, *flags.Srid)
	}
}
func TestEFlagIsParsed(t *testing.T) {
	expected := 32633
	os.Args = []string{"cesium_tiler", "-e=" + strconv.Itoa(expected)}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flags := tools.ParseFlags()
	if *flags.Srid != expected {
		t.Errorf("Expected Srid = %d, got %d", expected, *flags.Srid)
	}
}

func TestSridFlagDefaultIs4326(t *testing.T) {
	expected := 4326
	os.Args = []string{"cesium_tiler"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flags := tools.ParseFlags()
	if *flags.Srid != expected {
		t.Errorf("Expected Srid = %d, got %d", expected, *flags.Srid)
	}
}

func TestZOffsetFlagIsParsed(t *testing.T) {
	expected := 10.0
	os.Args = []string{"cesium_tiler", "-zoffset=10"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flags := tools.ParseFlags()
	if *flags.ZOffset != expected {
		t.Errorf("Expected ZOffset = %f, got %f", expected, *flags.ZOffset)
	}
}

func TestZFlagIsParsed(t *testing.T) {
	expected := 10.0
	os.Args = []string{"cesium_tiler", "-z=10"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flags := tools.ParseFlags()
	if *flags.ZOffset != expected {
		t.Errorf("Expected ZOffset = %f, got %f", expected, *flags.ZOffset)
	}
}

func TestZOffsetFlagDefaultIsZero(t *testing.T) {
	expected := 0.0
	os.Args = []string{"cesium_tiler"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flags := tools.ParseFlags()
	if *flags.ZOffset != expected {
		t.Errorf("Expected ZOffset = %f, got %f", expected, *flags.ZOffset)
	}
}

func TestMaxPtsFlagIsParsed(t *testing.T) {
	expected := 2000
	os.Args = []string{"cesium_tiler", "-maxpts=2000"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flags := tools.ParseFlags()
	if *flags.MaxNumPoints != expected {
		t.Errorf("Expected MaxNumPoints = %d, got %d", expected, *flags.MaxNumPoints)
	}
}
func TestMFlagIsParsed(t *testing.T) {
	expected := 2000
	os.Args = []string{"cesium_tiler", "-m=2000"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flags := tools.ParseFlags()
	if *flags.MaxNumPoints != expected {
		t.Errorf("Expected MaxNumPoints = %d, got %d", expected, *flags.MaxNumPoints)
	}
}

func TestMaxPtsFlagDefaultIs50000(t *testing.T) {
	expected := 50000
	os.Args = []string{"cesium_tiler"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flags := tools.ParseFlags()
	if *flags.MaxNumPoints != expected {
		t.Errorf("Expected MaxNumPoints = %d, got %d", expected, *flags.MaxNumPoints)
	}
}

func TestGeoidFlagIsParsed(t *testing.T) {
	expected := true
	os.Args = []string{"cesium_tiler", "-geoid"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flags := tools.ParseFlags()
	if !*flags.ZGeoidCorrection {
		t.Errorf("Expected ZGeoidCorrection = %t, got %t", expected, *flags.ZGeoidCorrection)
	}
}

func TestGFlagIsParsed(t *testing.T) {
	expected := true
	os.Args = []string{"cesium_tiler", "-g"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flags := tools.ParseFlags()
	if !*flags.ZGeoidCorrection {
		t.Errorf("Expected ZGeoidCorrection = %t, got %t", expected, *flags.ZGeoidCorrection)
	}
}

func TestGeoidFlagDefaultIsFalse(t *testing.T) {
	expected := false
	os.Args = []string{"cesium_tiler"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flags := tools.ParseFlags()
	if *flags.ZGeoidCorrection {
		t.Errorf("Expected ZGeoidCorrection = %t, got %t", expected, *flags.ZGeoidCorrection)
	}
}

func TestFolderProcessingFlagIsParsed(t *testing.T) {
	expected := true
	os.Args = []string{"cesium_tiler", "-folder"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flags := tools.ParseFlags()
	if !*flags.FolderProcessing {
		t.Errorf("Expected FolderProcessing = %t, got %t", expected, *flags.FolderProcessing)
	}
}

func TestFFlagIsParsed(t *testing.T) {
	expected := true
	os.Args = []string{"cesium_tiler", "-f"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flags := tools.ParseFlags()
	if !*flags.FolderProcessing {
		t.Errorf("Expected FolderProcessing = %t, got %t", expected, *flags.FolderProcessing)
	}
}

func TestFolderProcessingDefaultIsFalse(t *testing.T) {
	expected := false
	os.Args = []string{"cesium_tiler"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flags := tools.ParseFlags()
	if *flags.FolderProcessing {
		t.Errorf("Expected FolderProcessing = %t, got %t", expected, *flags.FolderProcessing)
	}
}

func TestRecursiveFolderProcessingFlagIsParsed(t *testing.T) {
	expected := true
	os.Args = []string{"cesium_tiler", "-recursive"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flags := tools.ParseFlags()
	if !*flags.RecursiveFolderProcessing {
		t.Errorf("Expected RecursiveFolderProcessing = %t, got %t", expected, *flags.RecursiveFolderProcessing)
	}
}

func TestRFlagIsParsed(t *testing.T) {
	expected := true
	os.Args = []string{"cesium_tiler", "-r"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flags := tools.ParseFlags()
	if !*flags.RecursiveFolderProcessing {
		t.Errorf("Expected RecursiveFolderProcessing = %t, got %t", expected, *flags.RecursiveFolderProcessing)
	}
}

func TestRecursiveFolderProcessingDefaultIsFalse(t *testing.T) {
	expected := false
	os.Args = []string{"cesium_tiler"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flags := tools.ParseFlags()
	if *flags.RecursiveFolderProcessing {
		t.Errorf("Expected RecursiveFolderProcessing = %t, got %t", expected, *flags.RecursiveFolderProcessing)
	}
}

func TestSilentFlagIsParsed(t *testing.T) {
	expected := true
	os.Args = []string{"cesium_tiler", "-silent"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flags := tools.ParseFlags()
	if !*flags.Silent {
		t.Errorf("Expected Silent = %t, got %t", expected, *flags.Silent)
	}
}

func TestSFlagIsParsed(t *testing.T) {
	expected := true
	os.Args = []string{"cesium_tiler", "-silent"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flags := tools.ParseFlags()
	if !*flags.Silent {
		t.Errorf("Expected Silent = %t, got %t", expected, *flags.Silent)
	}
}

func TestSilentFlagDefaultIsFalse(t *testing.T) {
	expected := false
	os.Args = []string{"cesium_tiler"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flags := tools.ParseFlags()
	if *flags.Silent {
		t.Errorf("Expected Silent = %t, got %t", expected, *flags.Silent)
	}
}

func TestLogTimestampFlagIsParsed(t *testing.T) {
	expected := true
	os.Args = []string{"cesium_tiler", "-timestamp"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flags := tools.ParseFlags()
	if !*flags.LogTimestamp {
		t.Errorf("Expected LogTimestamp = %t, got %t", expected, *flags.LogTimestamp)
	}
}

func TestTFlagIsParsed(t *testing.T) {
	expected := true
	os.Args = []string{"cesium_tiler", "-timestamp"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flags := tools.ParseFlags()
	if !*flags.LogTimestamp {
		t.Errorf("Expected LogTimestamp = %t, got %t", expected, *flags.LogTimestamp)
	}
}

func TestLogTimestampFlagDefaultIsFalse(t *testing.T) {
	expected := false
	os.Args = []string{"cesium_tiler"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flags := tools.ParseFlags()
	if *flags.LogTimestamp {
		t.Errorf("Expected LogTimestamp = %t, got %t", expected, *flags.LogTimestamp)
	}
}

func TestAlgorithmFlagIsParsed(t *testing.T) {
	expected := "random"
	os.Args = []string{"cesium_tiler", "-algorithm=random"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flags := tools.ParseFlags()
	if *flags.Algorithm != expected {
		t.Errorf("Expected Algorithm = %s, got %s", expected, *flags.Algorithm)
	}
}

func TestAlgorithmDefaultIsGrid(t *testing.T) {
	expected := "grid"
	os.Args = []string{"cesium_tiler"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flags := tools.ParseFlags()
	if *flags.Algorithm != "grid" {
		t.Errorf("Expected Algorithm = %s, got %s", expected, *flags.Algorithm)
	}
}

func TestGridMaxSizeFlagIsParsed(t *testing.T) {
	expected := 2.35
	os.Args = []string{"cesium_tiler", "-grid-max-size=2.35"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flags := tools.ParseFlags()
	if *flags.GridCellMaxSize != expected {
		t.Errorf("Expected Algorithm = %f, got %f", expected, *flags.GridCellMaxSize)
	}
}

func TestGridMaxSizeFlagDefaultIs5m(t *testing.T) {
	expected := 5.0
	os.Args = []string{"cesium_tiler"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flags := tools.ParseFlags()
	if *flags.GridCellMaxSize != expected {
		t.Errorf("Expected Algorithm = %f, got %f", expected, *flags.GridCellMaxSize)
	}
}

func TestGridMinSizeFlagIsParsed(t *testing.T) {
	expected := 0.04
	os.Args = []string{"cesium_tiler", "-grid-min-size=0.04"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flags := tools.ParseFlags()
	if *flags.GridCellMinSize != expected {
		t.Errorf("Expected Algorithm = %f, got %f", expected, *flags.GridCellMinSize)
	}
}

func TestGridMinSizeFlagDefaultIs15cm(t *testing.T) {
	expected := 0.15
	os.Args = []string{"cesium_tiler"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flags := tools.ParseFlags()
	if *flags.GridCellMinSize != expected {
		t.Errorf("Expected Algorithm = %f, got %f", expected, *flags.GridCellMinSize)
	}
}

func TestHelpFlagIsParsed(t *testing.T) {
	expected := true
	os.Args = []string{"cesium_tiler", "-help"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flags := tools.ParseFlags()
	if !*flags.Help {
		t.Errorf("Expected Help = %t, got %t", expected, *flags.Help)
	}
}

func TestHFlagIsParsed(t *testing.T) {
	expected := true
	os.Args = []string{"cesium_tiler", "-h"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flags := tools.ParseFlags()
	if !*flags.Help {
		t.Errorf("Expected Help = %t, got %t", expected, *flags.Help)
	}
}

func TestHelpDefaultIsFalse(t *testing.T) {
	expected := false
	os.Args = []string{"cesium_tiler"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flags := tools.ParseFlags()
	if *flags.Help {
		t.Errorf("Expected Help = %t, got %t", expected, *flags.Help)
	}
}

func TestRefineModeFlagIsParsed(t *testing.T) {
	expected := "REPLACE"
	os.Args = []string{"cesium_tiler", "-refine-mode=" + expected}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flags := tools.ParseFlags()
	if *flags.RefineMode != expected {
		t.Errorf("Expected Output = %s, got %s", expected, *flags.RefineMode)
	}
}

func TestRefineModeFlagDefaultIsAdd(t *testing.T) {
	expected := "ADD"
	os.Args = []string{"cesium_tiler"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flags := tools.ParseFlags()
	if *flags.RefineMode != expected {
		t.Errorf("Expected Output = %s, got %s", expected, *flags.RefineMode)
	}
}