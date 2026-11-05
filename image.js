// Draws a table as a PNG with AppKit, so macOS picks fonts for every script
// (Vietnamese, CJK, emoji). argv: spec.json ({header, rows, maxWidth}), out.png.
// Sizes are in points; the PNG is drawn at 2x and marked 144 dpi.
ObjC.import('AppKit');
function run(argv) {
  var spec = JSON.parse(ObjC.unwrap($.NSString.stringWithContentsOfFileEncodingError($(argv[0]), 4, null)));
  var out = argv[1];
  var rows = spec.header ? [spec.header].concat(spec.rows) : spec.rows;
  var padX = 12, padY = 8, scale = 2, maxW = spec.maxWidth > 0 ? spec.maxWidth : 100000;
  function color(hex) { var n = parseInt(hex.slice(1), 16); return $.NSColor.colorWithSRGBRedGreenBlueAlpha(((n>>16)&255)/255, ((n>>8)&255)/255, (n&255)/255, 1); }
  var font = $.NSFont.systemFontOfSize(14), bold = $.NSFont.boldSystemFontOfSize(14);
  var para = $.NSMutableParagraphStyle.alloc.init; para.lineBreakMode = 0;
  function attrs(isHead) { return $.NSDictionary.dictionaryWithObjectsForKeys($([isHead ? bold : font, color('#1f2328'), para]), $(['NSFont', 'NSColor', 'NSParagraphStyle'])); }
  var opts = (1 << 0) | (1 << 1); // uses line fragment origin | uses font leading
  var ncol = rows[0].length, widths = [], heights = [], sizes = [];
  for (var c = 0; c < ncol; c++) widths.push(0);
  rows.forEach(function (r, i) {
    var isHead = spec.header && i === 0; sizes.push([]);
    r.forEach(function (cell, c) {
      var s = $.NSString.alloc.initWithUTF8String(cell);
      var b = s.boundingRectWithSizeOptionsAttributes($.NSMakeSize(maxW, 100000), opts, attrs(isHead));
      var w = Math.ceil(b.size.width), h = Math.ceil(b.size.height);
      sizes[i].push(h); widths[c] = Math.max(widths[c], w);
    });
  });
  // Re-measure heights at the final column widths.
  rows.forEach(function (r, i) {
    var isHead = spec.header && i === 0, h = 0;
    r.forEach(function (cell, c) {
      var s = $.NSString.alloc.initWithUTF8String(cell);
      var b = s.boundingRectWithSizeOptionsAttributes($.NSMakeSize(widths[c], 100000), opts, attrs(isHead));
      h = Math.max(h, Math.ceil(b.size.height), Math.ceil(font.ascender - font.descender + font.leading));
    });
    heights.push(h + 2 * padY);
  });
  var W = 1, H = 1;
  widths.forEach(function (w) { W += w + 2 * padX + 1; });
  heights.forEach(function (h) { H += h + 1; });
  var rep = $.NSBitmapImageRep.alloc.initWithBitmapDataPlanesPixelsWidePixelsHighBitsPerSampleSamplesPerPixelHasAlphaIsPlanarColorSpaceNameBytesPerRowBitsPerPixel(null, W * scale, H * scale, 8, 4, true, false, 'NSCalibratedRGBColorSpace', 0, 0);
  rep.size = $.NSMakeSize(W, H);
  var ctx = $.NSGraphicsContext.graphicsContextWithBitmapImageRep(rep);
  $.NSGraphicsContext.saveGraphicsState; $.NSGraphicsContext.setCurrentContext(ctx);
  color('#ffffff').setFill; $.NSRectFill($.NSMakeRect(0, 0, W, H));
  var top = 0;
  rows.forEach(function (r, i) {
    var isHead = spec.header && i === 0, h = heights[i];
    var y = H - top - h - 1;
    if (isHead) { color('#f6f8fa').setFill; $.NSRectFill($.NSMakeRect(0, y, W, h + 1)); }
    var x = 1;
    r.forEach(function (cell, c) {
      var s = $.NSString.alloc.initWithUTF8String(cell);
      s.drawWithRectOptionsAttributes($.NSMakeRect(x + padX, y + padY, widths[c], h - 2 * padY), opts, attrs(isHead));
      x += widths[c] + 2 * padX + 1;
    });
    top += h + 1;
  });
  color('#d0d7de').setFill;
  var x = 0; $.NSRectFill($.NSMakeRect(0, 0, 1, H));
  widths.forEach(function (w) { x += w + 2 * padX + 1; $.NSRectFill($.NSMakeRect(x - 1, 0, 1, H)); });
  $.NSRectFill($.NSMakeRect(W - 1, 0, 1, H));
  var t = 0; $.NSRectFill($.NSMakeRect(0, H - 1, W, 1));
  heights.forEach(function (h) { t += h + 1; $.NSRectFill($.NSMakeRect(0, H - t - 1, W, 1)); });
  ctx.flushGraphics; $.NSGraphicsContext.restoreGraphicsState;
  var png = rep.representationUsingTypeProperties($.NSBitmapImageFileTypePNG, $({}));
  png.writeToFileAtomically($(out), true);
  return W + 'x' + H;
}
