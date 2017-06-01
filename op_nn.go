package gorgonia

import (
	"fmt"
	"hash"
	"hash/fnv"
	"time"

	"github.com/chewxy/gorgonia/tensor"
	"github.com/chewxy/hm"
	"github.com/leesper/go_rng"
	"github.com/pkg/errors"
)

/*
	This file contains all the Ops related to building a neural network.

	Bear in mind that not all things that are related to a neural network are here, as not everything
	are encoded as Ops the way theano does it.

	See also: nn.go for functions that relate to neural networks
*/

type randomness byte

const (
	uniform randomness = iota
	gaussian
	binomial
)

type randomOp struct {
	which randomness
	shape tensor.Shape
	dt    tensor.Dtype

	a, b float64 // when uniform, a,b = low, high; when gaussian, a,b = mean, stdev
}

func makeRandomOp(which randomness, dt tensor.Dtype, a, b float64, shape ...int) randomOp {
	return randomOp{
		which: which,
		shape: tensor.Shape(shape),
		dt:    dt,
		a:     a,
		b:     b,
	}
}

func (op randomOp) Arity() int { return 0 }

// randomOp :: a
// randomOp :: Tensor a
func (op randomOp) Type() hm.Type {
	if op.shape.IsScalar() {
		return op.dt
	}
	tt := newTensorType(op.shape.Dims(), op.dt)
	return tt
}

func (op randomOp) InferShape(...DimSizer) (tensor.Shape, error) { return op.shape, nil }

func (op randomOp) Do(...Value) (retVal Value, err error) {
	if op.shape.IsScalar() {
		var v interface{}
		switch op.dt {
		case Float64:
			switch op.which {
			case uniform:
				rand := rng.NewUniformGenerator(time.Now().UnixNano())
				v = rand.Float64Range(op.a, op.b)
			case gaussian:
				rand := rng.NewGaussianGenerator(time.Now().UnixNano())
				v = rand.Gaussian(op.a, op.b)
			case binomial:
				rand := rng.NewBinomialGenerator(time.Now().UnixNano())
				v = float64(rand.Binomial(int64(op.a), op.b))
			}
		case Float32:
			switch op.which {
			case uniform:
				rand := rng.NewUniformGenerator(time.Now().UnixNano())
				v = rand.Float32Range(float32(op.a), float32(op.b))
			case gaussian:
				rand := rng.NewGaussianGenerator(time.Now().UnixNano())
				v = float32(rand.Gaussian(op.a, op.b))
			case binomial:
				rand := rng.NewBinomialGenerator(time.Now().UnixNano())
				v = float32(rand.Binomial(int64(op.a), op.b))
			}
		default:
			return nil, errors.Errorf(nyiFail, "randomOp.do()", op.dt)
		}

		retVal, _ = anyToScalar(v)
		return
	}

	switch op.dt {
	case Float64:
		switch op.which {
		case uniform:
			backing := Uniform64(op.a, op.b, op.shape...)
			retVal = tensor.New(tensor.WithBacking(backing), tensor.WithShape(op.shape...))
		case gaussian:
			backing := Gaussian64(op.a, op.b, op.shape...)
			retVal = tensor.New(tensor.WithBacking(backing), tensor.WithShape(op.shape...))
		case binomial:
			backing := Binomial64(op.a, op.b, op.shape...)
			retVal = tensor.New(tensor.WithBacking(backing), tensor.WithShape(op.shape...))
		}
		return
	case Float32:
		switch op.which {
		case uniform:
			backing := Uniform32(op.a, op.b, op.shape...)
			retVal = tensor.New(tensor.WithBacking(backing), tensor.WithShape(op.shape...))
		case gaussian:
			backing := Gaussian32(op.a, op.b, op.shape...)
			retVal = tensor.New(tensor.WithBacking(backing), tensor.WithShape(op.shape...))
		case binomial:
			backing := Binomial32(op.a, op.b, op.shape...)
			retVal = tensor.New(tensor.WithBacking(backing), tensor.WithShape(op.shape...))
		}
		return
	default:
		return nil, errors.Errorf(nyiFail, "randomOp.do() for non-scalar", op.dt)
	}
}

func (op randomOp) ReturnsPtr() bool     { return false }
func (op randomOp) CallsExtern() bool    { return false }
func (op randomOp) OverwritesInput() int { return -1 }
func (op randomOp) WriteHash(h hash.Hash) {
	fmt.Fprintf(h, "%d%v%f%f", op.which, op.shape, op.a, op.b)
}

func (op randomOp) Hashcode() uint32 {
	h := fnv.New32a()
	op.WriteHash(h)
	return h.Sum32()
}

func (op randomOp) String() string {
	return fmt.Sprintf("%v(%v, %v) - %v", op.which, op.a, op.b, op.shape)
}

type im2colOp struct {
	h, w             int // kernel height and width
	padH, padW       int
	strideH, strideW int
}

func (op im2colOp) Arity() int { return 1 }

// im2col :: (Floats a) ⇒ Tensor a → Tensor a
func (op im2colOp) Type() hm.Type {
	tt := newTensorType(op.shape.Dims(), op.dt)
	return hm.NewFnType(tt, tt)
}

func (op im2colOp) InferShape(shapes ...DimSizer) (retVal tensor.Shape, err error) {
	if err = checkarity(op, len(shapes)); err != nil {
		return
	}

	var s tensor.Shape
	if s, err = shapes[0].(tensor.Shape); err != nil {
		return
	}

	return op.calcShape(s), nil
}

func (op im2ColOp) Do(inputs ...Value) (retVal Value, err error) {
	if err = checkArity(op, len(inputs)); err != nil {
		return
	}

	im := inputs[0]

	// todo type check values
	// todo shape check values

	retShape := op.calcShape(im.Shape())
	prealloc := tensor.New(tensor.Of(im.Dtype()), tensor.WithShape(retShape...))

	return op.do(prealloc, im)
}

func (op im2colOp) ReturnsPtr() bool     { return false }
func (op im2colOp) CallsExtern() bool    { return false }
func (op im2colOp) OverwritesInput() int { return -1 }

func (op im2colOp) WriteHash(h hash.Hash) {
	fmt.Fprintf(h, "im2col:%d-%d-%d-%d-%d-%d", op.h, op.w, op.padH, op.padW, op.strideH, op.StrideW)
}

func (op im2colOp) Hashcode() uint32 {
	h := fnv.New32a()
	op.WriteHash(h)
	return h.Sum32()
}

func (op randomOp) String() string {
	return fmt.Sprintf(h, "im2col<(%d,%d), (%d, %d), (%d,%d)>", op.h, op.w, op.padH, op.padW, op.strideH, op.StrideW)
}

func (op im2colOp) calcShape(s tensor.Shape) (retVal tensor.Shape) {
	b := s[0]
	c := s[1]
	h := s[2]
	w := s[3]

	h2 := (h+2*op.padH-op.h)/op.strideH + 1
	w2 := (w+2*op.padW-op.w)/op.strideW + 1
	retVal = tensor.Shape(tensor.BorrowInts(4))

	// todo: double check this with tests
	retVal[0] = b
	retVal[1] = h2
	retVal[2] = w2
	retVal[3] = c * op.w * op.h

	return
}

func (op im2colOp) do(prealloc, input Value) (retVal Value, err error) {
	// extract bchw - this bit can be expanded in the future, but for now we only support bchw
	s := input.Shape()
	b := s[0]
	c := s[1]
	h := s[2]
	w := s[3]

	switch input.Dtype() {
	case tensor.Float64:
		for i := 0; i < b; i++ {
			op.f64s(c, h, w, im.Data().([]float64), prealloc.Data().([]float64))
		}
	case tensor.Float32:
		for i := 0; i < b; i++ {
			// op.f32s(c, h, w, im.Data().([]float32), ...)
		}
	default:
		return err, nyiFail("im2col", input.Dtype())
	}
	return prealloc, nil
}

func (op im2colOp) f64s(channels, height, width int, im, col []float64) {
	retHeight := (height+2*op.padH-op.h)/op.strideH + 1
	retWidth := (width+2*op.padW-op.w)/op.strideW + 1
	retChans := channels * op.h * op.w

	for c := 0; c < retChans; c++ {
		widthOffset := c % op.w
		heightOffset := (c / op.w) % op.h
		imChan := c / op.h / op.w

		for h := 0; h < retHeight; h++ {
			for w := 0; w < retWidth; w++ {
				padH := h*op.strideH - op.padH + heightOffset
				padW := w*op.strideW - op.padW + widthOffset

				idx := retChans*chanWidths*h + retChans*w + c
				if padH >= 0 && padH < height && padW >= 0 && padW < width {
					imIdx := (retChans*height+padH)*width + padW
					col[idx] = im[imIdx]
				} else {
					col[idx] = 0
				}
			}
		}
	}
}

type col2imOp struct {
	h, w             int // patch height and width
	padH, padW       int
	strideH, strideW int
}

func (op col2imOp) f64s(channels, height, width int, col, im []floa64) {
	// memset im to 0
	for i := 0; i < height*width*channels; i++ {
		im[i] = 0
	}

	colHeight := (height+2*op.padH-op.h)/op.strideH + 1
	colWidth := (width+2*op.padW-op.w)/op.strideW + 1
	colChans := channels * op.h * op.w

	for c := 0; c < colChans; c++ {
		widthOffset := c % op.w
		heightOffset := (c / op.w) % op.h
		imChan := c / op.w / op.h
		for h := 0; h < colHeight; h++ {
			for w := 0; w < colWidth; w++ {
				padH := h*op.strideH - op.padH + heightOffset
				padW := w*op.stridew - op.padW + widthOffset
				if padH >= 0 && padH < height && padW > 0 && padW < width {
					imIdx := (imChan*height+padH)*width + padW
					colIdx := colChans*colWidth*h + colChans*w + c
					im[imIdx] += col[colIdx]
				}
			}
		}
	}
}
