// Copyright (C) The Lightning Authors. All rights reserved.
//
// SPDX-License-Identifier: AGPL-3.0

package hgvs

import (
	"fmt"
	"strings"
	"time"

	"github.com/sergi/go-diff/diffmatchpatch"
)

type Variant struct {
	Position int
	Ref      string
	New      string
	Left     string // base preceding an indel, if Ref or New is empty
}

func (v *Variant) String() string {
	switch {
	case len(v.New) == 0 && len(v.Ref) == 0:
		return fmt.Sprintf("%d=", v.Position)
	case len(v.New) == 0 && len(v.Ref) == 1:
		return fmt.Sprintf("%ddel", v.Position)
	case len(v.New) == 0:
		return fmt.Sprintf("%d_%ddel", v.Position, v.Position+len(v.Ref)-1)
	case len(v.Ref) == 1 && len(v.New) == 1:
		return fmt.Sprintf("%d%s>%s", v.Position, v.Ref, v.New)
	case len(v.Ref) == 0:
		return fmt.Sprintf("%d_%dins%s", v.Position-1, v.Position, v.New)
	case len(v.Ref) == 1 && len(v.New) > 0:
		return fmt.Sprintf("%ddelins%s", v.Position, v.New)
	default:
		return fmt.Sprintf("%d_%ddelins%s", v.Position, v.Position+len(v.Ref)-1, v.New)
	}
}

// PadLeft returns a Variant that is equivalent to v but (if possible)
// uses the stashed preceding base (the Left field) to avoid having a
// non-empty Ref or New part, even for an insertion or deletion.
//
// For example, if v is {Position: 45, Ref: "", New: "A"}, PadLeft
// might return {Position: 44, Ref: "T", New: "TA"}.
func (v *Variant) PadLeft() Variant {
	if len(v.Ref) == 0 || len(v.New) == 0 {
		return Variant{
			Position: v.Position - len(v.Left),
			Ref:      v.Left + v.Ref,
			New:      v.Left + v.New,
		}
	} else {
		return *v
	}
}

func Diff(a, b string, timeout time.Duration) ([]Variant, bool) {
	dmp := diffmatchpatch.New()
	var deadline time.Time
	if timeout > 0 {
		deadline = time.Now().Add(timeout)
	}
	diffs := dmp.DiffBisect(a, b, deadline)
	timedOut := false
	if timeout > 0 && time.Now().After(deadline) {
		timedOut = true
	}
	diffs = cleanup(dmp.DiffCleanupEfficiency(diffs))
	pos := 1
	var variants []Variant
	for i := 0; i < len(diffs); {
		left := "" // last char before an insertion or deletion
		for ; i < len(diffs) && diffs[i].Type == diffmatchpatch.DiffEqual; i++ {
			pos += len(diffs[i].Text)
			if tlen := len(diffs[i].Text); tlen > 0 {
				left = diffs[i].Text[tlen-1:]
			}
		}
		if i >= len(diffs) {
			break
		}
		v := Variant{Position: pos, Left: left}
		for ; i < len(diffs) && diffs[i].Type != diffmatchpatch.DiffEqual; i++ {
			if diffs[i].Type == diffmatchpatch.DiffDelete {
				v.Ref += diffs[i].Text
			} else {
				v.New += diffs[i].Text
			}
		}
		pos += len(v.Ref)
		variants = append(variants, v)
		left = ""
	}
	return variants, timedOut
}

func cleanup(in []diffmatchpatch.Diff) (out []diffmatchpatch.Diff) {
	out = make([]diffmatchpatch.Diff, 0, len(in))
	for i := 0; i < len(in); i++ {
		d := in[i]
		// Merge consecutive entries of same type (e.g.,
		// "insert A; insert B")
		for i < len(in)-1 && in[i].Type == in[i+1].Type {
			d.Text += in[i+1].Text
			i++
		}
		out = append(out, d)
	}
	in, out = out, make([]diffmatchpatch.Diff, 0, len(in))
	for i := 0; i < len(in); i++ {
		d := in[i]
		// diffmatchpatch solves diff("AAX","XTX") with
		// [delAA,=X,insTX] but we prefer to spell it
		// [delAA,insXT,=X].
		//
		// So, when we see a [del,=,ins] sequence where the
		// "=" part is a suffix of the "ins" part -- e.g.,
		// [delAAA,=CGG,insTTTCGG] -- we rearrange it to the
		// equivalent spelling [delAAA,insCGGTTT,=CGG].
		if i < len(in)-2 &&
			d.Type == diffmatchpatch.DiffDelete &&
			in[i+1].Type == diffmatchpatch.DiffEqual &&
			in[i+2].Type == diffmatchpatch.DiffInsert &&
			strings.HasSuffix(in[i+2].Text, in[i+1].Text) {
			eq, ins := in[i+1], in[i+2]
			ins.Text = eq.Text + ins.Text[:len(ins.Text)-len(eq.Text)]
			in[i+1] = ins
			in[i+2] = eq
		}
		// diffmatchpatch solves diff("AXX","XXX") with
		// [delA,=XX,insX] but we prefer to spell it
		// [delA,insX,=XX].
		//
		// So, when we see a [del,=,ins] sequence that has the
		// same effect after swapping the "=" and "ins" parts,
		// we swap them.
		if i < len(in)-2 &&
			d.Type == diffmatchpatch.DiffDelete &&
			in[i+1].Type == diffmatchpatch.DiffEqual &&
			in[i+2].Type == diffmatchpatch.DiffInsert &&
			in[i+1].Text+in[i+2].Text == in[i+2].Text+in[i+1].Text {
			in[i+2], in[i+1] = in[i+1], in[i+2]
		}
		// when diffmatchpatch says [delAAA, insXAY] and
		// len(X)==1, we prefer to treat the A>X as a snp.
		if i < len(in)-1 &&
			d.Type == diffmatchpatch.DiffDelete &&
			in[i+1].Type == diffmatchpatch.DiffInsert &&
			len(d.Text) > 2 &&
			len(in[i+1].Text) > 2 &&
			d.Text[1] == in[i+1].Text[1] {
			eqend := 2
			for ; eqend < len(d.Text) && eqend < len(in[i+1].Text) && d.Text[eqend] == in[i+1].Text[eqend]; eqend++ {
			}
			out = append(out,
				diffmatchpatch.Diff{diffmatchpatch.DiffDelete, d.Text[:1]},
				diffmatchpatch.Diff{diffmatchpatch.DiffInsert, in[i+1].Text[:1]},
				diffmatchpatch.Diff{diffmatchpatch.DiffEqual, d.Text[1:eqend]})
			in[i].Text, in[i+1].Text = in[i].Text[eqend:], in[i+1].Text[eqend:]
			i--
			continue
		}
		// when diffmatchpatch says [delAAA, insXaY] and
		// len(Y)==1, we prefer to treat the A>Y as a snp.
		if i < len(in)-1 &&
			d.Type == diffmatchpatch.DiffDelete &&
			in[i+1].Type == diffmatchpatch.DiffInsert &&
			len(d.Text) > 2 &&
			len(in[i+1].Text) > 2 &&
			d.Text[len(d.Text)-2] == in[i+1].Text[len(in[i+1].Text)-2] {
			// eqstart will be the number of equal chars
			// before the terminal snp, plus 1 for the snp
			// itself. Example, for [delAAAA, insTTAAG],
			// eqstart will be 3.
			eqstart := 2
			for ; eqstart < len(d.Text) && eqstart < len(in[i+1].Text) && d.Text[len(d.Text)-eqstart] == in[i+1].Text[len(in[i+1].Text)-eqstart]; eqstart++ {
			}
			eqstart--
			out = append(out,
				diffmatchpatch.Diff{diffmatchpatch.DiffDelete, d.Text[:len(d.Text)-eqstart]},
				diffmatchpatch.Diff{diffmatchpatch.DiffInsert, in[i+1].Text[:len(in[i+1].Text)-eqstart]},
				diffmatchpatch.Diff{diffmatchpatch.DiffEqual, d.Text[len(d.Text)-eqstart : len(d.Text)-1]},
				diffmatchpatch.Diff{diffmatchpatch.DiffDelete, d.Text[len(d.Text)-1:]},
				diffmatchpatch.Diff{diffmatchpatch.DiffInsert, in[i+1].Text[len(in[i+1].Text)-1:]})
			i++
			continue
		}
		out = append(out, d)
	}
	return
}

func Less(a, b Variant) bool {
	if a.Position != b.Position {
		return a.Position < b.Position
	} else if a.New != b.New {
		return a.New < b.New
	} else {
		return a.Ref < b.Ref
	}
}
