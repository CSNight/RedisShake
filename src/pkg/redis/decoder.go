// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package redis

import (
	"bufio"
	"bytes"
	"io"
	"strconv"

	"pkg/libs/errors"
	"pkg/libs/log"
)

var (
	ErrBadRespCRLFEnd  = errors.New("bad resp CRLF end")
	ErrBadRespBytesLen = errors.New("bad resp bytes len")
	ErrBadRespArrayLen = errors.New("bad resp array len")
)

type Decoder struct {
	r *bufio.Reader
}

func NewDecoder(r *bufio.Reader) *Decoder {
	return &Decoder{r: r}
}

func Decode(r *bufio.Reader) (Resp, error) {
	d := &Decoder{r}
	return d.decodeResp(0)
}

func MustDecodeOpt(d *Decoder) Resp {
	resp, err := d.decodeResp(0)
	if err != nil {
		log.PanicError(err, "decode redis resp failed")
	}
	return resp
}

func MustDecode(r *bufio.Reader) Resp {
	resp, err := Decode(r)
	if err != nil {
		log.PanicError(err, "decode redis resp failed")
	}
	return resp
}

func DecodeFromBytes(p []byte) (Resp, error) {
	r := bufio.NewReader(bytes.NewReader(p))
	return Decode(r)
}

func MustDecodeFromBytes(p []byte) Resp {
	resp, err := DecodeFromBytes(p)
	if err != nil {
		log.PanicError(err, "decode redis resp from bytes failed")
	}
	return resp
}

func (d *Decoder) decodeResp(depth int) (Resp, error) {
	t, err := d.decodeType()
	if err != nil {
		return nil, err
	}
	switch t {
	case typeString:
		resp := &String{}
		resp.Value, err = d.decodeText()
		return resp, err
	case typeError:
		resp := &Error{}
		resp.Value, err = d.decodeText()
		return resp, err
	case typeInt:
		resp := &Int{}
		resp.Value, err = d.decodeInt()
		return resp, err
	case typeBulkBytes:
		resp := &BulkBytes{}
		resp.Value, err = d.decodeBulkBytes()
		return resp, err
	case typeArray:
		resp := &Array{}
		resp.Value, err = d.decodeArray(depth)
		return resp, err
	default:
		if depth != 0 {
			return nil, errors.Errorf("bad resp type %s", t)
		}
		if err = d.r.UnreadByte(); err != nil {
			return nil, errors.Trace(err)
		}
		return d.decodeSingleLineBulkBytesArray()
	}
}

func (d *Decoder) decodeType() (respType, error) {
ReadByte:
	if b, err := d.r.ReadByte(); err != nil {
		return 0, errors.Trace(err)
	} else if string(b) == "\n" {
		/*
		 * Bugfix: see https://github.com/alibaba/RedisShake/issues/204.
		 * "\n" occurs before and after the +FULLRESYNC response sometimes at the redis version of 3.2.7.
		 */
		goto ReadByte
	} else {
		return respType(b), nil
	}
}

func (d *Decoder) decodeText() ([]byte, error) {
	b, err := d.r.ReadBytes('\n')
	if err != nil {
		return make([]byte, 0, 0), errors.Trace(err)
	}
	if n := len(b) - 2; n < 0 || b[n] != '\r' {
		return make([]byte, 0, 0), errors.Trace(ErrBadRespCRLFEnd)
	} else {
		//return string(b[:n]), nil
		return b[:n], nil
	}
}

func (d *Decoder) decodeInt() (int64, error) {
	b, err := d.decodeText()
	if err != nil {
		return 0, err
	}
	if n, err := strconv.ParseInt(string(b), 10, 64); err != nil {
		return 0, errors.Trace(err)
	} else {
		return n, nil
	}
}

func (d *Decoder) decodeBulkBytes() ([]byte, error) {
	n, err := d.decodeInt()
	if err != nil {
		return nil, err
	}
	if n < -1 {
		return nil, errors.Trace(ErrBadRespBytesLen)
	} else if n == -1 {
		return nil, nil
	}
	b := make([]byte, n+2)
	if _, err := io.ReadFull(d.r, b); err != nil {
		return nil, errors.Trace(err)
	}
	if b[n] != '\r' || b[n+1] != '\n' {
		return nil, errors.Trace(ErrBadRespCRLFEnd)
	}
	return b[:n], nil
}

func (d *Decoder) decodeArray(depth int) ([]Resp, error) {
	n, err := d.decodeInt()
	if err != nil {
		return nil, err
	}
	if n < -1 {
		return nil, errors.Trace(ErrBadRespArrayLen)
	} else if n == -1 {
		return nil, nil
	}
	a := make([]Resp, n)
	for i := 0; i < len(a); i++ {
		if a[i], err = d.decodeResp(depth + 1); err != nil {
			return nil, err
		}
	}
	return a, nil
}

func (d *Decoder) decodeSingleLineBulkBytesArray() (Resp, error) {
	b, err := d.r.ReadBytes('\n')
	if err != nil {
		return nil, errors.Trace(err)
	}
	if n := len(b) - 2; n < 0 || b[n] != '\r' {
		return nil, errors.Trace(ErrBadRespCRLFEnd)
	} else {
		resp := &Array{}
		for l, r := 0, 0; r <= n; r++ {
			if r == n || b[r] == ' ' {
				if l < r {
					resp.Value = append(resp.Value, &BulkBytes{b[l:r]})
				}
				l = r + 1
			}
		}
		return resp, nil
	}
}
