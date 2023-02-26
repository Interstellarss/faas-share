/*
Package binpack2d implements a two-dimensional rectangle bin packing algorithm.
It is loosely based on Jukka Jylänki's C++ implementation of RectangleBinPack->MaxRectsBinPack.
*/
package binpack2d

import "k8s.io/klog/v2"

// List of different heuristic rules that can be used when deciding where to place a new rectangle.
const (
  RULE_BEST_SHORT_SIDE_FIT  = iota
  RULE_BEST_LONG_SIDE_FIT
  RULE_BEST_AREA_FIT
  RULE_BOTTOM_LEFT
  RULE_CONTACT_POINT
  num_rules
)

// The Rectangle structure defines position and size of a rectangle.
type Rectangle struct { X, Y, W, H int }

// The Packer structure defines a single rectangular bin.
type Packer struct {
  width, height int   // bin dimension
  usedRects     []Rectangle // list of occupied space
  freeRects     []Rectangle // list of free space
  name string
}

func (p *Packer) DeepCopy() *Packer {
    newPacker := &Packer{
        width: p.width,
        height: p.height,
        usedRects: make([]Rectangle, len(p.usedRects)),
        freeRects: make([]Rectangle, len(p.freeRects)),
	name: p.name,
    }

    for i, r := range p.usedRects {
        newPacker.usedRects[i] = Rectangle{
            X: r.X,
            Y: r.Y,
            W: r.W,
            H: r.H,
        }
    }

    for i, r := range p.freeRects {
        newPacker.freeRects[i] = Rectangle{
            X: r.X,
            Y: r.Y,
            W: r.W,
            H: r.H,
        }
    }

    return newPacker
}

func (p *Packer) PrintInfo() {
    // Print packer information
    klog.Info("Packer: ", p.name, p.width, p.height)
    klog.Info("Used Rectangles:")
    for i, r := range p.usedRects {
	    klog.Info(" i,x,y,w,h:", i, r.X, r.Y, r.W, r.H)
    }
    klog.Info("Free Rectangles:\n")
    for i, r := range p.freeRects {
	    klog.Info(" i,x,y,w,h:", i, r.X, r.Y, r.W, r.H)
    }


}
func CreateWithName(width int, height int, name string) *Packer {
  p := Packer { 0, 0, make([]Rectangle, 0), make([]Rectangle, 0), name}
  p.Reset(width, height)
  return &p
}
// Create creates a new empty BinPacker structure of given dimension.
func Create(width, height int) *Packer {
  p := Packer { 0, 0, make([]Rectangle, 0), make([]Rectangle, 0), ""}
  p.Reset(width, height)
  return &p
}

// Reset removes all rectangles in the packer object and sets the bin size to the given dimension.
func (p *Packer) Reset(width, height int) {
  if width < 0 { width = 0 }
  if height < 0 { height = 0 }

  p.width = width
  p.height = height
  p.Clear()
}

// Clear removes all items from the list of used rectangles.
func (p *Packer) Clear() {
  p.usedRects = p.usedRects[:0]
  p.freeRects = p.freeRects[:0]
  addRect(&p.freeRects, len(p.freeRects), Rectangle{0, 0, p.width, p.height})
}

// GetWidth returns the width of the current bin.
func (p *Packer) GetWidth() int {
  return p.width
}

// GetHeight returns the height of the current bin.
func (p *Packer) GetHeight() int {
  return p.height
}

// GetUsedRectanglesLength returns the number of rectangles stored in the current bin.
func (p *Packer) GetUsedRectanglesLength() int {
  return len(p.usedRects)
}

// GetUsedRectangle returns the stored rectangle at the specified index. Returns an empty rectangle if the index is out of range.
func (p *Packer) GetUsedRectangle(index int) Rectangle {
  if index < 0 || index > len(p.usedRects) { return Rectangle{} }
  return p.usedRects[index]
}

// ShrinkBin attempts to shrink the current bin as much as possible. Use "binary" to specify whether to reduce dimensions by a fixed 50% per iteration.
func (p *Packer) ShrinkBin(binary bool) {
  if len(p.usedRects) == 0 { return }

  minX, minY, maxX, maxY := 1 << 30, 1 << 30, -(1 << 30), -(1 << 30)

  // finding borders
  for i := 0; i < len(p.usedRects); i++ {
    if p.usedRects[i].X < minX { minX = p.usedRects[i].X }
    if p.usedRects[i].Y < minY { minY = p.usedRects[i].Y }
    if p.usedRects[i].X + p.usedRects[i].W > maxX { maxX = p.usedRects[i].X + p.usedRects[i].W }
    if p.usedRects[i].Y + p.usedRects[i].H > maxY { maxY = p.usedRects[i].Y + p.usedRects[i].H }
  }

  newWidth, newHeight := maxX - minX, maxY - minY

  if binary {
    // attempt to shrink to the next lower power of two
    curWidth, curHeight := p.width, p.height
    for newWidth <= (curWidth >> 1) {
      curWidth >>= 1
    }
    newWidth = curWidth

    for newHeight <= (curHeight >> 1) {
      curHeight >>= 1
    }
    newHeight = curHeight
  }

  // adjusting rectangle positions
  if (newWidth != p.width || newHeight != p.height) && (minX > 0 || minY > 0) {
    for idx := 0; idx < len(p.freeRects); idx++ {
      p.freeRects[idx].X -= minX
      p.freeRects[idx].Y -= minY
    }
    for idx := 0; idx < len(p.usedRects); idx++ {
      p.usedRects[idx].X -= minX
      p.usedRects[idx].Y -= minY
    }
  }

  p.width = newWidth
  p.height = newHeight
}

func (p *Packer) TryInsert(width, height, rule int) (rect Rectangle, ok bool) {
  ok = false

  switch rule {
    case RULE_BEST_SHORT_SIDE_FIT:
      rect = p.findPositionForNewNodeBestShortSideFit(width, height, nil)
    case RULE_BOTTOM_LEFT:
      rect = p.findPositionForNewNodeBottomLeft(width, height, nil)
    case RULE_CONTACT_POINT:
      rect = p.findPositionForNewNodeContactPoint(width, height, nil)
    case RULE_BEST_LONG_SIDE_FIT:
      rect = p.findPositionForNewNodeBestLongSideFit(width, height, nil)
    case RULE_BEST_AREA_FIT:
      rect = p.findPositionForNewNodeBestAreaFit(width, height, nil)
    default:
      rect = Rectangle{}
      return
  }

  if rect.H == 0 {
    return
  }

  ok = true
  return
}



// Insert inserts a single rectangle to the bin by using the specified packing rule. 
// Returns the packed Rectangle structure, or sets "ok" to false if no fit could be found.
func (p *Packer) Insert(width, height, rule int) (rect Rectangle, ok bool) {
  ok = false

  switch rule {
    case RULE_BEST_SHORT_SIDE_FIT:
      rect = p.findPositionForNewNodeBestShortSideFit(width, height, nil)
    case RULE_BOTTOM_LEFT:
      rect = p.findPositionForNewNodeBottomLeft(width, height, nil)
    case RULE_CONTACT_POINT:
      rect = p.findPositionForNewNodeContactPoint(width, height, nil)
    case RULE_BEST_LONG_SIDE_FIT:
      rect = p.findPositionForNewNodeBestLongSideFit(width, height, nil)
    case RULE_BEST_AREA_FIT:
      rect = p.findPositionForNewNodeBestAreaFit(width, height, nil)
    default:
      rect = Rectangle{}
      return
  }

  if rect.H == 0 {
    return
  }

  for i, size := 0, len(p.freeRects); i < size; i++ {
    if p.splitFreeNode(i, rect) {
      removeRect(&p.freeRects, i)
      i--
      size--
    }
  }

  p.pruneFree()
  addRect(&p.usedRects, len(p.usedRects), rect)

  ok = true
  return
}

// GetOccupancy computes the ratio of used surface area to the total bin area.
func (p *Packer) GetOccupancy() float32 {
  usedSurfaceArea := int64(0)
  for i, size := 0, len(p.usedRects); i < size; i++ {
    usedSurfaceArea += int64(p.usedRects[i].W * p.usedRects[i].H)
  }

  return float32(usedSurfaceArea) / float32(p.width*p.height)
}


// Used internally. Computes the placement score for placing the given rectangle with the given method.
// width and height specify the rectangle dimension.
// rule specifies the placement rule.
// rect identifies where the rectangle would be placed if it were placed.
// pri and sec return the primary and secondary placement score.
// ok returns whether the rectangle fits into the bin.
func (p *Packer) scoreRect(width, height, rule int) (rect Rectangle, pri, sec int, ok bool) {
  ok = false
  pri, sec = 1 << 30, 1 << 30

  switch rule {
    case RULE_BEST_SHORT_SIDE_FIT:
      rect = p.findPositionForNewNodeBestShortSideFit(width, height, []int{pri, sec})
    case RULE_BOTTOM_LEFT:
      rect = p.findPositionForNewNodeBottomLeft(width, height, []int{pri, sec})
    case RULE_CONTACT_POINT:
      rect = p.findPositionForNewNodeContactPoint(width, height, []int{pri, sec})
    case RULE_BEST_LONG_SIDE_FIT:
      rect = p.findPositionForNewNodeBestLongSideFit(width, height, []int{pri, sec})
    case RULE_BEST_AREA_FIT:
      rect = p.findPositionForNewNodeBestAreaFit(width, height, []int{pri, sec})
    default:
      rect = Rectangle{}
      return
  }

  // cannot fit the current rectangle
  if rect.H == 0 {
    pri, sec = 1 << 30, 1 << 30
  } else {
    ok = true
  }

  return
}

// Used internally. Places the given rectangle into the bin.
func (p *Packer) placeRect(rect Rectangle) {
  for i, size := 0, len(p.freeRects); i < size; i++ {
    if p.splitFreeNode(i, rect) {
      removeRect(&p.freeRects, i)
      i--
      size--
    }
  }

  p.pruneFree()
  addRect(&p.usedRects, len(p.usedRects), rect)
}

// Used internally. Computes the placement score for the "CP" variant.
func (p *Packer) contactPointScoreNode(x, y, width, height int) int {
  score := 0

  if x == 0 || x + width == p.width {
    score += height
  }
  if y == 0 || y + height == p.height {
    score += width
  }

  for i, size := 0, len(p.usedRects); i < size; i++ {
    if p.usedRects[i].X == x + width || p.usedRects[i].X + p.usedRects[i].W == x {
      score += commonIntervalLength(p.usedRects[i].Y, p.usedRects[i].Y + p.usedRects[i].H, y, y + height)
    }
    if p.usedRects[i].Y == y + height || p.usedRects[i].Y + p.usedRects[i].H == y {
      score += commonIntervalLength(p.usedRects[i].X, p.usedRects[i].X + p.usedRects[i].W, x, x + width)
    }
  }

  return score
}

// Used internally. Implementing RULE_BOTTOM_LEFT packing rule.
func (p *Packer) findPositionForNewNodeBottomLeft(width, height int, bestPos []int) Rectangle {
  if bestPos == nil { bestPos = []int{0, 0} }
  bestNode := Rectangle{}

  bestPos[0] = 1 << 30
  for i, size := 0, len(p.freeRects); i < size; i++ {
    // Try to place the rectangle in upright (non-flipped) orientation.
    if p.freeRects[i].W >= width && p.freeRects[i].H >= height {
      topSideY := p.freeRects[i].Y + height
      if topSideY < bestPos[0] || (topSideY == bestPos[0] && p.freeRects[i].X < bestPos[1]) {
        bestNode.X, bestNode.Y = p.freeRects[i].X, p.freeRects[i].Y
        bestNode.W, bestNode.H = width, height
        bestPos[0], bestPos[1] = topSideY, p.freeRects[i].X
      }
    }
  }
  return bestNode
}

// Used internally. Implementing RULE_BEST_SHORT_SIDE_FIT packing rule.
func (p *Packer) findPositionForNewNodeBestShortSideFit(width, height int, bestFit []int) Rectangle {
  if bestFit == nil { bestFit = []int{0, 0} }
  bestNode := Rectangle{}

  bestFit[0] = 1 << 30
  for i, size := 0, len(p.freeRects); i < size; i++ {
    // Try to place the rectangle in upright (non-flipped) orientation.
    if p.freeRects[i].W >= width && p.freeRects[i].H >= height {
      leftoverHoriz := p.freeRects[i].W - width
      if leftoverHoriz < 0 { leftoverHoriz = -leftoverHoriz }
      leftoverVert := p.freeRects[i].H - height
      if leftoverVert < 0 { leftoverVert = -leftoverVert }
      shortSideFit := leftoverHoriz
      if leftoverVert < shortSideFit { shortSideFit = leftoverVert }
      longSideFit := leftoverHoriz
      if leftoverVert > longSideFit { longSideFit = leftoverVert }

      if shortSideFit < bestFit[0] || (shortSideFit == bestFit[0] && longSideFit < bestFit[1]) {
        bestNode.X, bestNode.Y = p.freeRects[i].X, p.freeRects[i].Y
        bestNode.W, bestNode.H = width, height
        bestFit[0], bestFit[1] = shortSideFit, longSideFit
      }
    }
  }
  return bestNode
}

// Used internally. Implementing RULE_BEST_LONG_SIDE_FIT packing rule.
func (p *Packer) findPositionForNewNodeBestLongSideFit(width, height int, bestFit []int) Rectangle {
  if bestFit == nil { bestFit = []int{0, 0} }
  bestNode := Rectangle{}

  bestFit[1] = 1 << 30
  for i, size := 0, len(p.freeRects); i < size; i++ {
    // Try to place the rectangle in upright (non-flipped) orientation.
    if p.freeRects[i].W >= width && p.freeRects[i].H >= height {
      leftoverHoriz := p.freeRects[i].W - width
      if leftoverHoriz < 0 { leftoverHoriz = -leftoverHoriz }
      leftoverVert := p.freeRects[i].H - height
      if leftoverVert < 0 { leftoverVert = -leftoverVert }
      shortSideFit := leftoverHoriz
      if leftoverVert < shortSideFit { shortSideFit = leftoverVert }
      longSideFit := leftoverHoriz
      if leftoverVert > longSideFit { longSideFit = leftoverVert }

      if longSideFit < bestFit[1] || (longSideFit == bestFit[1] && shortSideFit < bestFit[0]) {
        bestNode.X, bestNode.Y = p.freeRects[i].X, p.freeRects[i].Y
        bestNode.W, bestNode.H = width, height
        bestFit[0], bestFit[1] = shortSideFit, longSideFit
      }
    }
  }
  return bestNode
}

// Used internally. Implementing RULE_BEST_AREA_FIT packing rule.
func (p *Packer) findPositionForNewNodeBestAreaFit(width, height int, bestFit []int) Rectangle {
  if bestFit == nil { bestFit = []int{0, 0} }
  bestNode := Rectangle{}

  bestFit[0] = 1 << 30
  for i, size := 0, len(p.freeRects); i < size; i++ {
    areaFit := p.freeRects[i].W*p.freeRects[i].H - width*height

    // Try to place the rectangle in upright (non-flipped) orientation.
    if p.freeRects[i].W >= width && p.freeRects[i].H >= height {
      leftoverHoriz := p.freeRects[i].W - width
      if leftoverHoriz < 0 { leftoverHoriz = -leftoverHoriz }
      leftoverVert := p.freeRects[i].H - height
      if leftoverVert < 0 { leftoverVert = -leftoverVert }
      shortSideFit := leftoverHoriz
      if leftoverVert < shortSideFit { shortSideFit = leftoverVert }

      if areaFit < bestFit[0] || (areaFit == bestFit[0] && shortSideFit < bestFit[1]) {
        bestNode.X, bestNode.Y = p.freeRects[i].X, p.freeRects[i].Y
        bestNode.W, bestNode.H = width, height
        bestFit[0], bestFit[1] = areaFit, shortSideFit
      }
    }
  }
  return bestNode
}

// Used internally. Implementing RULE_CONTACT_POINT packing rule.
func (p *Packer) findPositionForNewNodeContactPoint(width, height int, bestScore []int) Rectangle {
  if bestScore == nil { bestScore = []int{0, 0} }
  bestNode := Rectangle{}

  bestScore[0] = -1
  for i, size := 0, len(p.freeRects); i < size; i++ {
    // Try to place the rectangle in upright (non-flipped) orientation.
    if p.freeRects[i].W >= width && p.freeRects[i].H >= height {
      score := p.contactPointScoreNode(p.freeRects[i].X, p.freeRects[i].Y, width, height)
      if score > bestScore[0] {
        bestNode.X, bestNode.Y = p.freeRects[i].X, p.freeRects[i].Y
        bestNode.W, bestNode.H = width, height
        bestScore[0] = score
      }
    }
  }
  return bestNode
}

// Used internally. Returns true if the free node was split.
func (p *Packer) splitFreeNode(freeIdx int, usedNode Rectangle) bool {
  // Test with SAT if the rectangles even intersect.
  if usedNode.X >= p.freeRects[freeIdx].X + p.freeRects[freeIdx].W ||
     usedNode.X + usedNode.W <= p.freeRects[freeIdx].X ||
     usedNode.Y >= p.freeRects[freeIdx].Y + p.freeRects[freeIdx].H ||
     usedNode.Y + usedNode.H <= p.freeRects[freeIdx].Y {
    return false
  }

  if usedNode.X < p.freeRects[freeIdx].X + p.freeRects[freeIdx].W && usedNode.X + usedNode.W > p.freeRects[freeIdx].X {
    // New node at the top side of the used node.
    if usedNode.Y > p.freeRects[freeIdx].Y && usedNode.Y < p.freeRects[freeIdx].Y + p.freeRects[freeIdx].H {
      newNode := Rectangle{p.freeRects[freeIdx].X, 
                           p.freeRects[freeIdx].Y, 
                           p.freeRects[freeIdx].W, 
                           usedNode.Y - p.freeRects[freeIdx].Y}
      addRect(&p.freeRects, len(p.freeRects), newNode)
    }

    // New node at the bottom side of the used node.
    if usedNode.Y + usedNode.H < p.freeRects[freeIdx].Y + p.freeRects[freeIdx].H {
      newNode := Rectangle{p.freeRects[freeIdx].X, 
                           usedNode.Y + usedNode.H, 
                           p.freeRects[freeIdx].W, 
                           p.freeRects[freeIdx].Y + p.freeRects[freeIdx].H - (usedNode.Y + usedNode.H)}
      addRect(&p.freeRects, len(p.freeRects), newNode)
    }
  }

  if usedNode.Y < p.freeRects[freeIdx].Y + p.freeRects[freeIdx].H && usedNode.Y + usedNode.H > p.freeRects[freeIdx].Y {
    // New node at the left side of the used node.
    if usedNode.X > p.freeRects[freeIdx].X && usedNode.X < p.freeRects[freeIdx].X + p.freeRects[freeIdx].W {
      newNode := Rectangle{p.freeRects[freeIdx].X, 
                           p.freeRects[freeIdx].Y, 
                           usedNode.X - p.freeRects[freeIdx].X, 
                           p.freeRects[freeIdx].H}
      addRect(&p.freeRects, len(p.freeRects), newNode)
    }

    // New node at the right side of the used node.
    if usedNode.X + usedNode.W < p.freeRects[freeIdx].X + p.freeRects[freeIdx].W {
      newNode := Rectangle{usedNode.X + usedNode.W, 
                           p.freeRects[freeIdx].Y, 
                           p.freeRects[freeIdx].X + p.freeRects[freeIdx].W - (usedNode.X + usedNode.W), 
                           p.freeRects[freeIdx].H}
      addRect(&p.freeRects, len(p.freeRects), newNode)
    }
  }
  return true
}

// Used internally. Goes through the free rectangle list and removes any redundant entries.
func (p *Packer) pruneFree() {
  // Go through each pair and remove any rectangle that is redundant.
  for i, size1 := 0, len(p.freeRects); i < size1; i++ {
    for j, size2 := i+1, len(p.freeRects); j < size2; j++ {
      if (isContainedIn(p.freeRects[i], p.freeRects[j])) {
        removeRect(&p.freeRects, i)
        i--
        size1--
        size2--
        break;
      }
      if (isContainedIn(p.freeRects[j], p.freeRects[i])) {
        removeRect(&p.freeRects, j)
        j--
        size1--
        size2--
      }
    }
  }
}

// Used internally. Returns 0 if the two intervals i1 and i2 are disjoint, or the length of their overlap otherwise.
func commonIntervalLength(i1start, i1end, i2start, i2end int) int {
  if i1end < i2start || i2end < i1start {
    return 0
  }
  end := i1end
  if i2end < end { end = i2end }
  start := i1start
  if i2start > start { start = i2start }
  return end - start
}

// Used internally. Returns true if a is contained in b.
func isContainedIn(a, b Rectangle) bool {
  return a.X >= b.X && a.Y >= b.Y && a.X+a.W <= b.X+b.W && a.Y+a.H <= b.Y+b.H;
}

// Used internally. Adds a new rectangle to the rectangle list at the specified position.
func addRect(list *[]Rectangle, index int, rect Rectangle) {
  if index < 0 { index = 0 }
  if index > len(*list) { index = len(*list) }

  *list = append(*list, Rectangle{})
  for pos := len(*list) - 1; pos > index; pos-- {
    (*list)[pos] = (*list)[pos - 1]
  }
  (*list)[index] = rect
}

// Used internally. Removes the rectangle from the given list at the specified position.
func removeRect(list *[]Rectangle, index int) {
  if index < 0 { index = 0 }
  if index >= len(*list) { index = len(*list) }

  for pos := index + 1; pos < len(*list); pos++ {
    (*list)[pos - 1] = (*list)[pos]
  }
  *list = (*list)[:len(*list) - 1]
}
