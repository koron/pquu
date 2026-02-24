package gitapply

import (
	"bytes"
	"fmt"
	"unicode"

	"github.com/bluekeyes/go-gitdiff/gitdiff"
)

type image struct {
	buf  []byte
	line []lineInfo
}

type lineInfo struct {
	offset int
	len    int
	hash   uint32
	flag   int
}

const (
	lineCommon  = 1
	linePatched = 2
)

func hashLine(buf []byte) uint32 {
	var h uint32
	for _, b := range buf {
		if !unicode.IsSpace(rune(b)) {
			h = h*3 + uint32(b)
		}
	}
	return h
}

func newImage(buf []byte) *image {
	img := &image{buf: buf}
	img.prepareLineTable()
	return img
}

func (img *image) prepareLineTable() {
	img.line = nil
	offset := 0
	for offset < len(img.buf) {
		next := offset
		for next < len(img.buf) && img.buf[next] != '\n' {
			next++
		}
		if next < len(img.buf) {
			next++
		}
		lineBuf := img.buf[offset:next]
		img.line = append(img.line, lineInfo{
			offset: offset,
			len:    len(lineBuf),
			hash:   hashLine(lineBuf),
		})
		offset = next
	}
}

func (img *image) removeFirstLine() {
	if len(img.line) == 0 {
		return
	}
	l := img.line[0]
	img.buf = img.buf[l.len:]
	img.line = img.line[1:]
	for i := range img.line {
		img.line[i].offset -= l.len
	}
}

func (img *image) removeLastLine() {
	if len(img.line) == 0 {
		return
	}
	l := img.line[len(img.line)-1]
	img.buf = img.buf[:len(img.buf)-l.len]
	img.line = img.line[:len(img.line)-1]
}

type applyState struct {
	pContext       int
	allowOverlap   bool
	applyInReverse bool
	reject         bool
}

func (s *applyState) applyOneFragment(img *image, frag *gitdiff.TextFragment, nth int) error {
	var preimage, postimage image
	var oldlines, newlines bytes.Buffer

	for _, l := range frag.Lines {
		op := l.Op
		if s.applyInReverse {
			if op == gitdiff.OpAdd {
				op = gitdiff.OpDelete
			} else if op == gitdiff.OpDelete {
				op = gitdiff.OpAdd
			}
		}

		lineStr := l.Line
		// go-gitdiff lines might not have \n if it was a "No newline at end of file"
		// but Git's apply.c expects them or handles them.
		// Actually gitdiff.Line.Line usually includes the newline if it was in the source.
		// Wait, let me check go-gitdiff documentation about Line.Line.
		// "Lines read by ReadLinesAt include the newline character."

		switch op {
		case gitdiff.OpContext:
			oldlines.WriteString(lineStr)
			newlines.WriteString(lineStr)
			preimage.line = append(preimage.line, lineInfo{
				offset: preimage.offset(),
				len:    len(lineStr),
				hash:   hashLine([]byte(lineStr)),
				flag:   lineCommon,
			})
			postimage.line = append(postimage.line, lineInfo{
				offset: postimage.offset(),
				len:    len(lineStr),
				hash:   hashLine([]byte(lineStr)),
				flag:   lineCommon,
			})
		case gitdiff.OpDelete:
			oldlines.WriteString(lineStr)
			preimage.line = append(preimage.line, lineInfo{
				offset: preimage.offset(),
				len:    len(lineStr),
				hash:   hashLine([]byte(lineStr)),
				flag:   0,
			})
		case gitdiff.OpAdd:
			newlines.WriteString(lineStr)
			postimage.line = append(postimage.line, lineInfo{
				offset: postimage.offset(),
				len:    len(lineStr),
				hash:   hashLine([]byte(lineStr)),
				flag:   0,
			})
		}
	}
	preimage.buf = oldlines.Bytes()
	postimage.buf = newlines.Bytes()

	leading := int(frag.LeadingContext)
	trailing := int(frag.TrailingContext)

	// matchBeginning and matchEnd logic
	matchBeginning := frag.OldPosition <= 1
	matchEnd := trailing == 0 // Simplified

	pos := int(frag.OldPosition)
	if pos > 0 {
		pos--
	}

	appliedPos := -1
	for {
		appliedPos = s.findPos(img, &preimage, &postimage, pos, matchBeginning, matchEnd)
		if appliedPos >= 0 {
			break
		}

		// Context reduction
		if leading <= s.pContext && trailing <= s.pContext {
			break
		}
		if matchBeginning || matchEnd {
			matchBeginning = false
			matchEnd = false
			continue
		}

		if leading >= trailing {
			preimage.removeFirstLine()
			postimage.removeFirstLine()
			pos--
			leading--
		}
		if trailing > leading {
			preimage.removeLastLine()
			postimage.removeLastLine()
			trailing--
		}
	}

	if appliedPos >= 0 {
		s.updateImage(img, appliedPos, &preimage, &postimage)
		return nil
	}

	return fmt.Errorf("patch failed: hunk #%d", nth)
}

func (img *image) offset() int {
	if len(img.line) == 0 {
		return 0
	}
	last := img.line[len(img.line)-1]
	return last.offset + last.len
}

func (s *applyState) findPos(img *image, preimage, postimage *image, line int, matchBeginning, matchEnd bool) int {
	if matchBeginning && matchEnd && len(img.line) != len(preimage.line) {
		matchBeginning = false
	}

	if matchBeginning {
		line = 0
	} else if matchEnd {
		line = len(img.line) - len(preimage.line)
	}

	if line < 0 {
		line = 0
	}
	if line > len(img.line) {
		line = len(img.line)
	}

	backwardsLno := line
	forwardsLno := line

	for i := 0; ; i++ {
		currLno := -1
		if i%2 == 0 {
			if forwardsLno <= len(img.line) {
				currLno = forwardsLno
				forwardsLno++
			} else if backwardsLno >= 0 {
				i++
				currLno = backwardsLno
				backwardsLno--
			}
		} else {
			if backwardsLno >= 0 {
				currLno = backwardsLno
				backwardsLno--
			} else if forwardsLno <= len(img.line) {
				i++
				currLno = forwardsLno
				forwardsLno++
			}
		}

		if currLno == -1 || (backwardsLno < 0 && forwardsLno > len(img.line)) {
			break
		}

		if currLno >= 0 && currLno <= len(img.line) {
			if s.matchFragment(img, preimage, postimage, currLno, matchBeginning, matchEnd) {
				return currLno
			}
		}
	}

	return -1
}

func (s *applyState) matchFragment(img *image, preimage, postimage *image, line int, matchBeginning, matchEnd bool) bool {
	if line+len(preimage.line) > len(img.line) {
		return false
	}
	if matchBeginning && line != 0 {
		return false
	}
	if matchEnd && line+len(preimage.line) != len(img.line) {
		return false
	}

	for i := 0; i < len(preimage.line); i++ {
		if (img.line[line+i].flag & linePatched) != 0 {
			return false
		}
		if img.line[line+i].hash != preimage.line[i].hash {
			return false
		}
	}

	// Exact match check
	var offset int
	if line < len(img.line) {
		offset = img.line[line].offset
	} else {
		offset = len(img.buf)
	}

	if offset+len(preimage.buf) > len(img.buf) {
		return false
	}

	if !bytes.Equal(img.buf[offset:offset+len(preimage.buf)], preimage.buf) {
		return false
	}

	return true
}

func (s *applyState) updateImage(img *image, appliedPos int, preimage, postimage *image) {
	var appliedAt int
	if appliedPos < len(img.line) {
		appliedAt = img.line[appliedPos].offset
	} else {
		appliedAt = len(img.buf)
	}

	removeCount := 0
	for i := 0; i < len(preimage.line); i++ {
		removeCount += img.line[appliedPos+i].len
	}

	newBuf := make([]byte, 0, len(img.buf)-removeCount+len(postimage.buf))
	newBuf = append(newBuf, img.buf[:appliedAt]...)
	newBuf = append(newBuf, postimage.buf...)
	newBuf = append(newBuf, img.buf[appliedAt+removeCount:]...)

	img.buf = newBuf

	// Update line table
	newLineInfos := make([]lineInfo, len(postimage.line))
	copy(newLineInfos, postimage.line)
	for i := range newLineInfos {
		newLineInfos[i].offset += appliedAt
		if !s.allowOverlap {
			newLineInfos[i].flag |= linePatched
		}
	}

	oldLineNr := len(img.line)
	preimageLineNr := len(preimage.line)
	postimageLineNr := len(postimage.line)

	resLine := make([]lineInfo, 0, oldLineNr-preimageLineNr+postimageLineNr)
	resLine = append(resLine, img.line[:appliedPos]...)
	resLine = append(resLine, newLineInfos...)
	resLine = append(resLine, img.line[appliedPos+preimageLineNr:]...)

	// Update offsets for the remaining lines
	diff := len(postimage.buf) - removeCount
	for i := appliedPos + postimageLineNr; i < len(resLine); i++ {
		resLine[i].offset += diff
	}

	img.line = resLine
}

func applyPatch(src []byte, f *gitdiff.File, opts ApplyOptions) ([]byte, []int, error) {
	if f.IsBinary {
		if f.BinaryFragment != nil {
			if f.BinaryFragment.Method == gitdiff.BinaryPatchLiteral {
				return f.BinaryFragment.Data, nil, nil
			}
			// For BinaryPatchDelta, we would need to implement Git's binary delta application.
			// Since it's complex, we'll return an error for now or fallback if possible.
			return nil, nil, fmt.Errorf("binary delta patch not supported in this port")
		}
		if f.IsDelete {
			return nil, nil, nil
		}
		return src, nil, nil
	}

	img := newImage(src)
	state := &applyState{
		pContext: 0, // Default context to match
		reject:   opts.Reject,
	}

	var rejectedIndices []int
	for i, frag := range f.TextFragments {
		err := state.applyOneFragment(img, frag, i+1)
		if err != nil {
			if state.reject {
				rejectedIndices = append(rejectedIndices, i)
				continue
			}
			return nil, nil, err
		}
	}

	return img.buf, rejectedIndices, nil
}
