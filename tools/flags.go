package tools

import (
	"flag"
	"log"
)

const (
	CommandIndex          = "index"
	CommandMergeChildren  = "merge-children"
	CommandMergeTree      = "merge-tree"
	CommandVerifyLas      = "verify-las"
	CommandVerifyLasMerge = "verify-las-merge"
)

type FlagsGlobal struct {
	Help    *bool `json:"help"`
	Version *bool `json:"version"`
}

type TilerFlags struct {
	Input                     *string `json:"input"`
	Srid                      *int    `json:"srid"`
	EightBitColors            *bool
	ZOffset                   *float64
	MaxNumPoints              *int
	MinNumPoints              *int
	ZGeoidCorrection          *bool
	FolderProcessing          *bool
	RecursiveFolderProcessing *bool
	Algorithm                 *string
	GridCellMaxSize           *float64 `json:"grid_max_size"`
	GridCellMinSize           *float64 `json:"grid_min_size"`
	RefineMode                *string  `json:"refine_mode"`
	Draco                     *bool
}

type FlagsForCommandIndex struct {
	TilerFlags
	Output                         *string
	UseEdgeCalculateGeometricError *bool
	Silent                         *bool
	LogTimestamp                   *bool
	Help                           *bool
	Version                        *bool
}

type FlagsForCommandMerge struct {
	TilerFlags
}

type FlagsForCommandVerify struct {
	TilerFlags
	Output      *string
	OffsetBegin *int
	OffsetEnd   *int
}

func ParseFlagsGlobal() FlagsGlobal {
	help := defineBoolFlag("help", "h", false, "Displays this help.")
	version := defineBoolFlag("version", "v", false, "Displays the version of gocesiumtiler.")

	flag.Parse()

	return FlagsGlobal{
		Help:    help,
		Version: version,
	}
}

func ParseFlagsForCommandIndex(args []string) FlagsForCommandIndex {
	log.Println(FmtJSONString(args))

	flagCommand := flag.NewFlagSet("command-index", flag.ExitOnError)

	input := defineStringFlagCommand(flagCommand, "input", "i", "", "Specifies the input las file/folder.")
	output := defineStringFlagCommand(flagCommand, "output", "o", "", "Specifies the output folder where to write the tileset data.")
	srid := defineIntFlagCommand(flagCommand, "srid", "e", 4326, "EPSG srid code of input points.")
	eightBit := defineBoolFlagCommand(flagCommand, "8bit", "b", false, "Assumes the input LAS has colors encoded in eight bit format. Default is false (LAS has 16 bit color depth)")
	zOffset := defineFloat64FlagCommand(flagCommand, "zoffset", "z", 0, "Vertical offset to apply to points, in meters.")
	zGeoidCorrection := defineBoolFlagCommand(flagCommand, "geoid", "g", false, "Enables Geoid to Ellipsoid elevation correction. Use this flag if your input LAS files have Z coordinates specified relative to the Earth geoid rather than to the standard ellipsoid.")
	minNumPointsPerNode := defineIntFlagCommand(flagCommand, "points-min-num", "m", 10000, "Minimum allowed number of points per node for GridTree Algorithms.")
	folderProcessing := defineBoolFlagCommand(flagCommand, "folder", "f", false, "Enables processing of all las files from input folder. Input must be a folder if specified")
	recursiveFolderProcessing := defineBoolFlagCommand(flagCommand, "recursive", "r", false, "Enables recursive lookup for all .las files inside the subfolders")
	gridCellMaxSize := defineFloat64FlagCommand(flagCommand, "grid-max-size", "x", 5.0, "Max cell size in meters for the grid algorithm. It roughly represents the max spacing between any two samples. ")
	gridCellMinSize := defineFloat64FlagCommand(flagCommand, "grid-min-size", "n", 0.15, "Min cell size in meters for the grid algorithm. It roughly represents the minimum possible size of a 3d tile. ")
	refineMode := defineStringFlagCommand(flagCommand, "refine-mode", "", "ADD", "Type of refine mode, can be 'ADD' or 'REPLACE'. 'ADD' means that child tiles will not contain the parent tiles points. 'REPLACE' means that they will also contain the parent tiles points. ADD implies less disk space but more network overhead when fetching the data, REPLACE is the opposite.")
	draco := defineBoolFlagCommand(flagCommand, "draco", "", false, "Use Draco algorithm to compress xyz and color")

	useEdgeCalculateGeometricError := defineBoolFlagCommand(flagCommand, "use-edge-calculate", "d", true, "Assumes use chunk-edge x/y/z to calculate tileset geometricError")
	silent := defineBoolFlagCommand(flagCommand, "silent", "s", false, "Use to suppress all the non-error messages.")
	logTimestamp := defineBoolFlagCommand(flagCommand, "timestamp", "t", false, "Adds timestamp to log messages.")
	help := defineBoolFlagCommand(flagCommand, "help", "h", false, "Displays this help.")
	version := defineBoolFlagCommand(flagCommand, "version", "v", false, "Displays the version of gocesiumtiler.")

	maxNumPointsPerNode := 50000
	algorithm := "grid"

	flagCommand.Parse(args)

	return FlagsForCommandIndex{
		TilerFlags: TilerFlags{
			Input:                     input,
			Srid:                      srid,
			EightBitColors:            eightBit,
			ZOffset:                   zOffset,
			MaxNumPoints:              &maxNumPointsPerNode,
			MinNumPoints:              minNumPointsPerNode,
			ZGeoidCorrection:          zGeoidCorrection,
			FolderProcessing:          folderProcessing,
			RecursiveFolderProcessing: recursiveFolderProcessing,
			Algorithm:                 &algorithm,
			GridCellMaxSize:           gridCellMaxSize,
			GridCellMinSize:           gridCellMinSize,
			RefineMode:                refineMode,
			Draco:                     draco,
		},
		Output:                         output,
		UseEdgeCalculateGeometricError: useEdgeCalculateGeometricError,
		Silent:                         silent,
		LogTimestamp:                   logTimestamp,
		Help:                           help,
		Version:                        version,
	}
}

func ParseFlagsForCommandMerge(args []string) FlagsForCommandMerge {
	log.Println(FmtJSONString(args))

	flagCommand := flag.NewFlagSet("command-merge", flag.ExitOnError)

	input := defineStringFlagCommand(flagCommand, "input", "i", "", "Specifies the input tileset parent folder.")
	srid := defineIntFlagCommand(flagCommand, "srid", "e", 4326, "EPSG srid code of input points.")
	eightBit := defineBoolFlagCommand(flagCommand, "8bit", "b", false, "Assumes the input LAS has colors encoded in eight bit format. Default is false (LAS has 16 bit color depth)")
	zOffset := defineFloat64FlagCommand(flagCommand, "zoffset", "z", 0, "Vertical offset to apply to points, in meters.")
	zGeoidCorrection := defineBoolFlagCommand(flagCommand, "geoid", "g", false, "Enables Geoid to Ellipsoid elevation correction. Use this flag if your input LAS files have Z coordinates specified relative to the Earth geoid rather than to the standard ellipsoid.")
	recursiveFolderProcessing := defineBoolFlagCommand(flagCommand, "recursive", "r", false, "Enables recursive lookup for all .las files inside the subfolders")
	gridCellMaxSize := defineFloat64FlagCommand(flagCommand, "grid-max-size", "x", 10.0, "Max cell size in meters for the grid algorithm. It roughly represents the max spacing between any two samples. ")
	gridCellMinSize := defineFloat64FlagCommand(flagCommand, "grid-min-size", "n", 5.0, "Min cell size in meters for the grid algorithm. It roughly represents the minimum possible size of a 3d tile. ")
	refineMode := defineStringFlagCommand(flagCommand, "refine-mode", "", "ADD", "Type of refine mode, can be 'ADD' or 'REPLACE'. 'ADD' means that child tiles will not contain the parent tiles points. 'REPLACE' means that they will also contain the parent tiles points. ADD implies less disk space but more network overhead when fetching the data, REPLACE is the opposite.")
	draco := defineBoolFlagCommand(flagCommand, "draco", "", false, "Use Draco algorithm to compress xyz and color")

	folderProcessing := true

	minNumPointsPerNode := 10000
	maxNumPointsPerNode := 50000
	algorithm := "grid"

	flagCommand.Parse(args)

	return FlagsForCommandMerge{
		TilerFlags: TilerFlags{
			Input:                     input,
			Srid:                      srid,
			EightBitColors:            eightBit,
			ZOffset:                   zOffset,
			MaxNumPoints:              &maxNumPointsPerNode,
			MinNumPoints:              &minNumPointsPerNode,
			ZGeoidCorrection:          zGeoidCorrection,
			FolderProcessing:          &folderProcessing,
			RecursiveFolderProcessing: recursiveFolderProcessing,
			Algorithm:                 &algorithm,
			GridCellMaxSize:           gridCellMaxSize,
			GridCellMinSize:           gridCellMinSize,
			RefineMode:                refineMode,
			Draco:                     draco,
		},
	}
}

func ParseFlagsForCommandVerify(args []string) FlagsForCommandVerify {
	log.Println(FmtJSONString(args))

	flagCommand := flag.NewFlagSet("command-verify", flag.ExitOnError)

	input := defineStringFlagCommand(flagCommand, "input", "i", "", "Specifies the input tileset parent folder.")
	output := defineStringFlagCommand(flagCommand, "output", "o", "", "Specifies the output folder where to write the tileset data.")
	srid := defineIntFlagCommand(flagCommand, "srid", "e", 4326, "EPSG srid code of input points.")
	eightBit := defineBoolFlagCommand(flagCommand, "8bit", "b", false, "Assumes the input LAS has colors encoded in eight bit format. Default is false (LAS has 16 bit color depth)")
	zOffset := defineFloat64FlagCommand(flagCommand, "zoffset", "z", 0, "Vertical offset to apply to points, in meters.")
	zGeoidCorrection := defineBoolFlagCommand(flagCommand, "geoid", "g", false, "Enables Geoid to Ellipsoid elevation correction. Use this flag if your input LAS files have Z coordinates specified relative to the Earth geoid rather than to the standard ellipsoid.")
	recursiveFolderProcessing := defineBoolFlagCommand(flagCommand, "recursive", "r", false, "Enables recursive lookup for all .las files inside the subfolders")
	gridCellMaxSize := defineFloat64FlagCommand(flagCommand, "grid-max-size", "x", 10.0, "Max cell size in meters for the grid algorithm. It roughly represents the max spacing between any two samples. ")
	gridCellMinSize := defineFloat64FlagCommand(flagCommand, "grid-min-size", "n", 5.0, "Min cell size in meters for the grid algorithm. It roughly represents the minimum possible size of a 3d tile. ")

	offsetBegin := 0
	offsetEnd := -1

	folderProcessing := true

	minNumPointsPerNode := 10000
	maxNumPointsPerNode := 50000
	algorithm := "grid"
	refineMode := "REPLACE"

	flagCommand.Parse(args)

	return FlagsForCommandVerify{
		TilerFlags: TilerFlags{
			Input:                     input,
			Srid:                      srid,
			EightBitColors:            eightBit,
			ZOffset:                   zOffset,
			MaxNumPoints:              &maxNumPointsPerNode,
			MinNumPoints:              &minNumPointsPerNode,
			ZGeoidCorrection:          zGeoidCorrection,
			FolderProcessing:          &folderProcessing,
			RecursiveFolderProcessing: recursiveFolderProcessing,
			Algorithm:                 &algorithm,
			GridCellMaxSize:           gridCellMaxSize,
			GridCellMinSize:           gridCellMinSize,
			RefineMode:                &refineMode,
		},
		Output:      output,
		OffsetBegin: &offsetBegin,
		OffsetEnd:   &offsetEnd,
	}
}

func defineStringFlag(name string, shortHand string, defaultValue string, usage string) *string {
	var output string
	flag.StringVar(&output, name, defaultValue, usage)
	if shortHand != name && shortHand != "" {
		flag.StringVar(&output, shortHand, defaultValue, usage+" (shorthand for "+name+")")
	}

	return &output
}

func defineIntFlag(name string, shortHand string, defaultValue int, usage string) *int {
	var output int
	flag.IntVar(&output, name, defaultValue, usage)
	if shortHand != name {
		flag.IntVar(&output, shortHand, defaultValue, usage+" (shorthand for "+name+")")
	}

	return &output
}

func defineFloat64Flag(name string, shortHand string, defaultValue float64, usage string) *float64 {
	var output float64
	flag.Float64Var(&output, name, defaultValue, usage)
	if shortHand != name {
		flag.Float64Var(&output, shortHand, defaultValue, usage+" (shorthand for "+name+")")
	}
	return &output
}

func defineBoolFlag(name string, shortHand string, defaultValue bool, usage string) *bool {
	var output bool
	flag.BoolVar(&output, name, defaultValue, usage)
	if shortHand != name {
		flag.BoolVar(&output, shortHand, defaultValue, usage+" (shorthand for "+name+")")
	}
	return &output
}

func defineStringFlagCommand(flagCommand *flag.FlagSet, name string, shortHand string, defaultValue string, usage string) *string {
	var output string
	flagCommand.StringVar(&output, name, defaultValue, usage)
	if shortHand != name && shortHand != "" {
		flagCommand.StringVar(&output, shortHand, defaultValue, usage+" (shorthand for "+name+")")
	}

	return &output
}

func defineIntFlagCommand(flagCommand *flag.FlagSet, name string, shortHand string, defaultValue int, usage string) *int {
	var output int
	flagCommand.IntVar(&output, name, defaultValue, usage)
	if shortHand != name {
		flagCommand.IntVar(&output, shortHand, defaultValue, usage+" (shorthand for "+name+")")
	}

	return &output
}

func defineFloat64FlagCommand(flagCommand *flag.FlagSet, name string, shortHand string, defaultValue float64, usage string) *float64 {
	var output float64
	flagCommand.Float64Var(&output, name, defaultValue, usage)
	if shortHand != name {
		flagCommand.Float64Var(&output, shortHand, defaultValue, usage+" (shorthand for "+name+")")
	}
	return &output
}

func defineBoolFlagCommand(flagCommand *flag.FlagSet, name string, shortHand string, defaultValue bool, usage string) *bool {
	var output bool
	flagCommand.BoolVar(&output, name, defaultValue, usage)
	if shortHand != name {
		flagCommand.BoolVar(&output, shortHand, defaultValue, usage+" (shorthand for "+name+")")
	}
	return &output
}
