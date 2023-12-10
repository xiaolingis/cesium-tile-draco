package ply

import (
	"unsafe"

	plyfile "github.com/cobaltgray/go-plyfile"
)

type Vertex struct {
	X, Y, Z float32
	R, G, B uint8
}

func SetPlyProperties() (vertProps []plyfile.PlyProperty, faceProps []plyfile.PlyProperty) {
	vertProps = make([]plyfile.PlyProperty, 6)
	vertProps[0] = plyfile.PlyProperty{"x", plyfile.PLY_FLOAT, plyfile.PLY_FLOAT, int(unsafe.Offsetof(Vertex{}.X)), 0, 0, 0, 0}
	vertProps[1] = plyfile.PlyProperty{"y", plyfile.PLY_FLOAT, plyfile.PLY_FLOAT, int(unsafe.Offsetof(Vertex{}.Y)), 0, 0, 0, 0}
	vertProps[2] = plyfile.PlyProperty{"z", plyfile.PLY_FLOAT, plyfile.PLY_FLOAT, int(unsafe.Offsetof(Vertex{}.Z)), 0, 0, 0, 0}
	vertProps[3] = plyfile.PlyProperty{"red", plyfile.PLY_UCHAR, plyfile.PLY_UCHAR, int(unsafe.Offsetof(Vertex{}.R)), 0, 0, 0, 0}
	vertProps[4] = plyfile.PlyProperty{"green", plyfile.PLY_UCHAR, plyfile.PLY_UCHAR, int(unsafe.Offsetof(Vertex{}.G)), 0, 0, 0, 0}
	vertProps[5] = plyfile.PlyProperty{"blue", plyfile.PLY_UCHAR, plyfile.PLY_UCHAR, int(unsafe.Offsetof(Vertex{}.B)), 0, 0, 0, 0}

	faceProps = make([]plyfile.PlyProperty, 0)

	return vertProps, faceProps
}

func WritePlyFile(filePath string, verts []Vertex) error {
	elem_names := make([]string, 2)
	elem_names[0] = "vertex"
	elem_names[1] = "face"

	var version float32

	//log.Printf("Writing PLY file 'test.ply'...")

	cplyfile := plyfile.PlyOpenForWriting(filePath, len(elem_names), elem_names, plyfile.PLY_ASCII, &version)
	vertProps, _ := SetPlyProperties()

	// Describe vertex properties
	plyfile.PlyElementCount(cplyfile, "vertex", len(verts))
	plyfile.PlyDescribeProperty(cplyfile, "vertex", vertProps[0])
	plyfile.PlyDescribeProperty(cplyfile, "vertex", vertProps[1])
	plyfile.PlyDescribeProperty(cplyfile, "vertex", vertProps[2])
	plyfile.PlyDescribeProperty(cplyfile, "vertex", vertProps[3])
	plyfile.PlyDescribeProperty(cplyfile, "vertex", vertProps[4])
	plyfile.PlyDescribeProperty(cplyfile, "vertex", vertProps[5])

	// Add a comment and an object information field
	plyfile.PlyPutComment(cplyfile, "Generated by WangJian")
	plyfile.PlyPutObjInfo(cplyfile, "Generated by WangJian")

	// Finish writing header
	plyfile.PlyHeaderComplete(cplyfile)

	// Setup and write vertex elements
	plyfile.PlyPutElementSetup(cplyfile, "vertex")
	for _, vertex := range verts {
		plyfile.PlyPutElement(cplyfile, vertex)
	}
	// close the PLY file
	plyfile.PlyClose(cplyfile)

	//log.Printf("Wrote PLY file.")
	return nil
}