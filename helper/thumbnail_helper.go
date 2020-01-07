package thumbnail

import (
	"bytes"
	"image"
	"image/jpeg"
	"io"
	"os"

	"github.com/bakape/thumbnailer"
	th "github.com/bakape/thumbnailer"
	"github.com/oliamb/cutter"
)

//ThumbSize declares the Height and Width of the thumb-square
const ThumbSize = 128

//Thumbnail creates a thumbnail from the passed file reader
func Thumbnail(file *os.File) io.Reader {
	rs := io.ReadSeeker(file)
	var src th.Source
	var thumb image.Image
	//get original dimensions
	ctx, _ := thumbnailer.NewFFContext(rs)
	src.Dims, _ = ctx.Dims()
	ctx.Close()
	//calc dimension to fit smalles side to ThumbSize
	opts := thumbnailer.Options{
		ThumbDims: calcRatio(src.Dims),
	}
	//thumbnail image
	src, thumb, _ = thumbnailer.Process(rs, opts)
	//crop image to centered square
	thumb, _ = cutter.Crop(thumb, cutter.Config{
		Width:   1,
		Height:  1,
		Mode:    cutter.Centered,
		Options: cutter.Ratio, // Copy is useless here
	})
	//Encode Image with compression
	var opt = jpeg.Options{
		Quality: 80,
	}
	//write thumb into buffer
	buff := new(bytes.Buffer)
	err := jpeg.Encode(buff, thumb, &opt)
	if err != nil {
		panic(err)
	}
	//return buffer as reader
	return bytes.NewReader(buff.Bytes())
}

func calcRatio(dims thumbnailer.Dims) thumbnailer.Dims {
	if dims.Width == dims.Height {
		return dims
	} else if dims.Width > dims.Height {
		tmp := ThumbSize / float64(dims.Height)
		tmp = float64(dims.Width) * tmp
		return thumbnailer.Dims{
			Width:  uint(tmp),
			Height: ThumbSize + 2,
		}
	} else {
		tmp := ThumbSize / float64(dims.Width)
		tmp = float64(dims.Height) * tmp
		return thumbnailer.Dims{
			Width:  ThumbSize + 2,
			Height: uint(tmp),
		}
	}
}
