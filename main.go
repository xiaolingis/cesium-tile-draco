/*
 * This file is part of the Go Cesium Point Cloud Tiler distribution (https://github.com/ecopia-map/cesium_tiler).
 * Copyright (c) 2023 Ecopia Alpaca - ecopia-alpaca@ecopiax.com
 *
 * This program is free software; you can redistribute it and/or modify it
 * under the terms of the GNU Lesser General Public License Version 3 as
 * published by the Free Software Foundation;
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the GNU
 * Lesser General Public License for more details.
 *
 * You should have received a copy of the GNU Lesser General Public License
 * along with this program. If not, see <http://www.gnu.org/licenses/>.
 *
 * This software also uses third party components. You can find information
 * on their credits and licensing in the file LICENSE-3RD-PARTIES.md that
 * you should have received togheter with the source code.
 */

package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ecopia-map/cesium_tiler/internal/tiler"
	"github.com/ecopia-map/cesium_tiler/pkg"
	"github.com/ecopia-map/cesium_tiler/pkg/algorithm_manager/std_algorithm_manager"
	"github.com/ecopia-map/cesium_tiler/tools"
	"github.com/golang/glog"
	// "github.com/pkg/profile" // enable for profiling
)

const version = "v2.0.0"

const logo = `
               _                    _   _ _
  ___ ___  ___(_)_   _ _ __ ___    | |_(_) | ___ _ __
 / __/ _ \/ __| | | | | '_   _ \   | __| | |/ _ \ '__|
| (_|  __/\__ \ | |_| | | | | | |--| |_| | |  __/ |
 \___\___||___/_|\__,_|_| |_| |_|-- \__|_|_|\___|_|
  A Cesium Point Cloud tile generator written in golang
  Copyright 2023 - Ecopia Alpaca
`

var codeVersion string

func main() {
	InitGlog()
	defer glog.Flush()

	flagsGlobal := tools.ParseFlagsGlobal()
	glog.Infoln(tools.FmtJSONString(flagsGlobal))

	// Prints the command line flag description
	if *flagsGlobal.Help {
		showHelp()
		return
	}

	if *flagsGlobal.Version {
		printVersion()
		return
	}

	args := flag.Args()
	if len(args) == 0 {
		glog.Fatal("Please specify a subcommand [index|merge].")
	}
	cmd, args := args[0], args[1:]

	switch cmd {
	case tools.CommandIndex:
		mainCommandIndex(args)
	case tools.CommandMergeChildren:
		mainCommandMerge(args, cmd)
	case tools.CommandMergeTree:
		mainCommandMerge(args, cmd)
	case tools.CommandVerifyLas:
		mainCommandVerifyLas(args, cmd)
	case tools.CommandVerifyLasMerge:
		mainCommandVerifyLas(args, cmd)
	default:
		glog.Fatalf("Unrecognized command [%q]. Command must be one of [index|merge]", cmd)
	}

}

func mainCommandIndex(args []string) {
	// remove comment to enable the profiler (remember to remove comment in the imports)
	// defer profile.Start(profile.MemProfileRate(1)).Stop()

	// Retrieve command line args
	flags := tools.ParseFlagsForCommandIndex(args)

	// Prints the command line flag description
	if *flags.Help {
		showHelpForSubCommand(flags.FlagCommand)
		return
	}

	if *flags.Version {
		printVersion()
		return
	}

	glog.Infoln("flags", tools.FmtJSONString(flags))

	tilerFlags := flags.TilerFlags

	// Put args inside a TilerOptions struct
	opts := tiler.TilerOptions{
		Input:                  *tilerFlags.Input,
		Srid:                   *tilerFlags.Srid,
		EightBitColors:         *tilerFlags.EightBitColors,
		ZOffset:                *tilerFlags.ZOffset,
		MaxNumPointsPerNode:    int32(*tilerFlags.MaxNumPoints),
		MinNumPointsPerNode:    int32(*tilerFlags.MinNumPoints),
		EnableGeoidZCorrection: *tilerFlags.ZGeoidCorrection,
		FolderProcessing:       *tilerFlags.FolderProcessing,
		Recursive:              *tilerFlags.RecursiveFolderProcessing,
		Algorithm:              tiler.Algorithm(strings.ToUpper(*tilerFlags.Algorithm)),
		CellMinSize:            *tilerFlags.GridCellMinSize,
		CellMaxSize:            *tilerFlags.GridCellMaxSize,
		RefineMode:             tiler.ParseRefineMode(*tilerFlags.RefineMode),
		Draco:                  *tilerFlags.Draco,
		DracoEncoderPath:       *tilerFlags.DracoEncoderPath,

		Command: tools.CommandIndex,
		TilerIndexOptions: &tiler.TilerIndexOptions{
			Output:                         *flags.Output,
			UseEdgeCalculateGeometricError: *flags.UseEdgeCalculateGeometricError,
		},
	}

	// Validate TilerOptions
	if msg, res := validateOptionsForCommandIndex(&opts, &flags); !res {
		glog.Fatal("Error parsing input parameters: " + msg)
	}

	// Starts the tiler
	// defer timeTrack(time.Now(), "tiler")
	err := pkg.NewTiler(tools.NewStandardFileFinder(), std_algorithm_manager.NewAlgorithmManager(&opts)).RunTiler(&opts)

	if err != nil {
		glog.Fatal("Error while tiling: ", err)
	} else {
		glog.Infoln("Conversion Completed")
	}
}

// Validates the input options provided to the command line tool checking
// that input and output folders/files exist
func validateOptionsForCommandIndex(opts *tiler.TilerOptions, flags *tools.FlagsForCommandIndex) (string, bool) {
	if _, err := os.Stat(opts.Input); os.IsNotExist(err) {
		return "Input file/folder not found", false
	}
	if _, err := os.Stat(*flags.Output); os.IsNotExist(err) {
		return "Output folder not found", false
	}

	if opts.CellMinSize > opts.CellMaxSize {
		return "grid-max-size parameter cannot be lower than grid-min-size parameter", false
	}

	if opts.RefineMode == "" {
		return "refine-mode should be either ADD or REPLACE", false
	}

	if opts.Draco && opts.DracoEncoderPath == "" {
		return "draco-encoder-path must be set", false
	}

	return "", true
}

func mainCommandMerge(args []string, cmd string) {
	flags := tools.ParseFlagsForCommandMerge(args)

	// Prints the command line flag description
	if *flags.Help {
		showHelpForSubCommand(flags.FlagCommand)
		return
	}

	if *flags.Version {
		printVersion()
		return
	}

	glog.Infoln("flags", tools.FmtJSONString(flags))

	tilerFlags := flags.TilerFlags

	// Put args inside a TilerOptions struct
	opts := tiler.TilerOptions{
		Command:                cmd,
		Input:                  *tilerFlags.Input,
		Srid:                   *tilerFlags.Srid,
		EightBitColors:         *tilerFlags.EightBitColors,
		ZOffset:                *tilerFlags.ZOffset,
		MaxNumPointsPerNode:    int32(*tilerFlags.MaxNumPoints),
		MinNumPointsPerNode:    int32(*tilerFlags.MinNumPoints),
		EnableGeoidZCorrection: *tilerFlags.ZGeoidCorrection,
		FolderProcessing:       *tilerFlags.FolderProcessing,
		Recursive:              *tilerFlags.RecursiveFolderProcessing,
		Algorithm:              tiler.Algorithm(strings.ToUpper(*tilerFlags.Algorithm)),
		CellMinSize:            *tilerFlags.GridCellMinSize,
		CellMaxSize:            *tilerFlags.GridCellMaxSize,
		RefineMode:             tiler.ParseRefineMode(*tilerFlags.RefineMode),
		Draco:                  *tilerFlags.Draco,
		DracoEncoderPath:       *tilerFlags.DracoEncoderPath,

		TilerMergeOptions: &tiler.TilerMergeOptions{
			Output: "",
		},
	}

	// Validate TilerOptions
	if msg, res := validateOptionsForCommandMerge(&opts, &flags); !res {
		glog.Fatal("Error parsing input parameters: " + msg)
	}

	// Starts the tiler
	// defer timeTrack(time.Now(), "tiler")
	fileFinder := tools.NewStandardFileFinder()
	algorithmManager := std_algorithm_manager.NewAlgorithmManager(&opts)
	err := pkg.NewTilerMerge(fileFinder, algorithmManager).RunTiler(&opts)

	if err != nil {
		glog.Fatal("Error while tiling: ", err)
	} else {
		glog.Infoln("Conversion Completed")
	}

}

func validateOptionsForCommandMerge(opts *tiler.TilerOptions, flags *tools.FlagsForCommandMerge) (string, bool) {
	if _, err := os.Stat(opts.Input); os.IsNotExist(err) {
		return "Input file/folder not found", false
	}

	if opts.RefineMode == "" {
		return "refine-mode should be either ADD or REPLACE", false
	}

	return "", true
}

func mainCommandVerifyLas(args []string, cmd string) {
	flags := tools.ParseFlagsForCommandVerify(args)

	// Prints the command line flag description
	if *flags.Help {
		showHelpForSubCommand(flags.FlagCommand)
		return
	}

	if *flags.Version {
		printVersion()
		return
	}

	glog.Infoln("flags", tools.FmtJSONString(flags))

	tilerFlags := flags.TilerFlags

	// Put args inside a TilerOptions struct
	opts := tiler.TilerOptions{
		Command:                cmd,
		Input:                  *tilerFlags.Input,
		Srid:                   *tilerFlags.Srid,
		EightBitColors:         *tilerFlags.EightBitColors,
		ZOffset:                *tilerFlags.ZOffset,
		MaxNumPointsPerNode:    int32(*tilerFlags.MaxNumPoints),
		MinNumPointsPerNode:    int32(*tilerFlags.MinNumPoints),
		EnableGeoidZCorrection: *tilerFlags.ZGeoidCorrection,
		FolderProcessing:       *tilerFlags.FolderProcessing,
		Recursive:              *tilerFlags.RecursiveFolderProcessing,
		Algorithm:              tiler.Algorithm(strings.ToUpper(*tilerFlags.Algorithm)),
		CellMinSize:            *tilerFlags.GridCellMinSize,
		CellMaxSize:            *tilerFlags.GridCellMaxSize,
		RefineMode:             tiler.ParseRefineMode(*tilerFlags.RefineMode),
		TilerVerifyOptions: &tiler.TilerVerifyOptions{
			Output:      "",
			OffsetBegin: 0,
			OffsetEnd:   -1,
		},
	}

	// Validate TilerOptions
	if msg, res := validateOptionsForCommandVerify(&opts, &flags); !res {
		glog.Fatal("Error parsing input parameters: " + msg)
	}

	// Starts the tiler
	// defer timeTrack(time.Now(), "tiler")
	fileFinder := tools.NewStandardFileFinder()
	algorithmManager := std_algorithm_manager.NewAlgorithmManager(&opts)
	err := pkg.NewTilerVerify(fileFinder, algorithmManager).RunTiler(&opts)

	if err != nil {
		glog.Fatal("Error while tiling: ", err)
	} else {
		glog.Infoln("Conversion Completed")
	}

}

func validateOptionsForCommandVerify(opts *tiler.TilerOptions, flags *tools.FlagsForCommandVerify) (string, bool) {
	if _, err := os.Stat(opts.Input); os.IsNotExist(err) {
		return "Input file/folder not found", false
	}

	if opts.RefineMode == "" {
		return "refine-mode should be either ADD or REPLACE", false
	}

	return "", true
}

func timeTrack(start time.Time, name string) {
	elapsed := time.Since(start)
	glog.Infoln(fmt.Sprintf("%s took %s", name, elapsed))
}

func printLogo() {
	fmt.Println(strings.ReplaceAll(logo, "YYYY", strconv.Itoa(time.Now().Year())))
}

func showHelp() {
	printLogo()
	fmt.Println("***")
	fmt.Println("CesiumTiler is a tool that processes LAS files and transforms them in a 3D Tiles data structure consumable by Cesium.js")
	printVersion()
	fmt.Println("***")
	fmt.Println("")
	fmt.Println("Usage: ./cesium_tiler < index | merge-tree | merge-children | verify-las | verify-las-merge >")
	fmt.Println("")
	fmt.Println("Command line flags: ")
	flag.CommandLine.SetOutput(os.Stdout)
	flag.PrintDefaults()
}

func showHelpForSubCommand(flagCommand *flag.FlagSet) {
	printLogo()
	fmt.Println("***")
	fmt.Println("CesiumTiler is a tool that processes LAS files and transforms them in a 3D Tiles data structure consumable by Cesium.js")
	printVersion()
	fmt.Println("***")
	fmt.Println("")
	fmt.Println("Command line flags: ")
	flagCommand.SetOutput(os.Stdout)
	flagCommand.PrintDefaults()
}

func printVersion() {
	fmt.Printf("%s-%s\n", version, codeVersion)
}

func GetCodeVersion() string {
	if len(codeVersion) >= 8 {
		return codeVersion[:8]
	} else if len(codeVersion) > 0 {
		return codeVersion
	} else {
		return "Unknow"
	}

}

func InitGlog() {
	flag.Set("logtostderr", "true")
	flag.Set("stderrthreshold", "WARNING")
	flag.Set("v", "2")
	flag.Parse()
}
