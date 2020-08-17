# Copyright 2020 The SQLFlow Authors. All rights reserved.
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

import io

from matplotlib._pylab_helpers import Gcf
from matplotlib.backend_bases import FigureManagerBase
from matplotlib.backends.backend_agg import FigureCanvasAgg, RendererAgg
from PIL import Image
from plotille import Canvas


class RendererPlotille(RendererAgg):
    def __init__(self, width, height, dpi):
        super(RendererPlotille, self).__init__(width, height, dpi)
        self.texts = []

    def clear(self):
        super(RendererPlotille, self).clear()
        self.texts = []

    def draw_text(self, gc, x, y, s, prop, angle, ismath=False, mtext=None):
        self.texts.append((x, y, s))


def show():
    try:
        for manager in Gcf.get_all_fig_managers():
            canvas = manager.canvas
            canvas.draw()
            string = canvas.to_txt()
            print(string)
    finally:
        pass


class FigureCanvasPlotille(FigureCanvasAgg):
    def print_figure(self, fname, **kwargs):
        try:
            for manager in Gcf.get_all_fig_managers():
                canvas = manager.canvas
                canvas.draw()
                string = canvas.to_txt()
                print(string,
                      file=open(fname + '.txt', 'w', encoding="utf-8"),
                      end='')
        finally:
            pass

    def get_renderer(self, cleared=False):
        l, b, w, h = self.figure.bbox.bounds
        key = w, h, self.figure.dpi
        reuse_renderer = (hasattr(self, "renderer")
                          and getattr(self, "_lastKey", None) == key)
        if not reuse_renderer:
            self.renderer = RendererPlotille(w, h, self.figure.dpi)
            self._lastKey = key
        elif cleared:
            self.renderer.clear()
        return self.renderer

    def to_txt(self):
        # import pdb; pdb.set_trace()

        buf = io.BytesIO()
        self.print_png(buf)
        buf.seek(0)

        i = Image.open(buf).convert('RGB')

        w, h = i.size

        can = Canvas(128, int(80 / w * h), color_mode='byte')

        for y in range(h):
            for x in range(w):
                center = i.getpixel((x, y))
                if center == (255, 255, 255):
                    continue
                if x in range(1, w - 1) and y in range(1, h - 1):
                    # Use the most deepest color in a 3x3 area as
                    # the center color
                    surrounding = i.getpixel((x, y - 1))  # upper
                    center = surrounding if grayscale(center) > grayscale(
                        surrounding) else center
                    surrounding = i.getpixel((x, y + 1))  # lower
                    center = surrounding if grayscale(center) > grayscale(
                        surrounding) else center
                    surrounding = i.getpixel((x - 1, y))  # left
                    center = surrounding if grayscale(center) > grayscale(
                        surrounding) else center
                    surrounding = i.getpixel((x + 1, y))  # right
                    center = surrounding if grayscale(center) > grayscale(
                        surrounding) else center
                    surrounding = i.getpixel((x - 1, y - 1))  # upper left
                    center = surrounding if grayscale(center) > grayscale(
                        surrounding) else center
                    surrounding = i.getpixel((x + 1, y - 1))  # upper right
                    center = surrounding if grayscale(center) > grayscale(
                        surrounding) else center
                    surrounding = i.getpixel((x - 1, y + 1))  # lower left
                    center = surrounding if grayscale(center) > grayscale(
                        surrounding) else center
                    surrounding = i.getpixel((x + 1, y + 1))  # lower right
                    center = surrounding if grayscale(center) > grayscale(
                        surrounding) else center
                color = closest_term256_color(center)
                if grayscale(center) < 200:  # Ignore background color
                    can.point(float(x) / w, 1 - float(y) / h, True, color)

        def set_text(canvas, x, y, text):
            x_idx = canvas._transform_x(x)
            y_idx = canvas._transform_y(y)
            x_c = max(x_idx // 2, 0)
            y_c = max(y_idx // 4, 0)

            for i, c in enumerate(text):
                canvas._canvas[y_c][x_c + i] = c

        for x, y, s in self.renderer.texts:
            set_text(can, float(x) / w, 1 - float(y) / h, s)

        txt = can.plot()
        upper_margin = '⠀' * 128 + '\n'
        while txt.startswith(upper_margin):
            txt = txt[len(upper_margin):]
        return txt.rstrip('⠀ \n')  # Remove bottom margin


color_map = {}


# Convert RGB to grayscale, see
# https://www.tutorialspoint.com/dip/grayscale_to_rgb_conversion.htm
def grayscale(rgb):
    return 0.3 * rgb[0] + 0.59 * rgb[1] + 0.11 * rgb[2]


# Compute distance between two RGB color
def distance(c1, c2):
    c1r, c1g, c1b = c1
    c2r, c2g, c2b = c2

    dr = c1r - c2r
    dg = c1g - c2g
    db = c1b - c2b
    return dr * dr + dg * dg + db * db


# Find the closest xterm 256 colors
def closest_term256_color(pixel):
    best = color_map.get(pixel, None)
    bestdist = 0.0
    for i, v in enumerate(colors):
        dist = distance(pixel, v)
        if best is None or dist < bestdist:
            best = i
            bestdist = dist
    color_map[pixel] = best
    return best


FigureCanvas = FigureCanvasPlotille
FigureManager = FigureManagerBase

# Mapping xterm 256 colors to RGB, see https://jonasjacek.github.io/colors/
colors = [
    (0, 0, 0),
    (128, 0, 0),
    (0, 128, 0),
    (128, 128, 0),
    (0, 0, 128),
    (128, 0, 128),
    (0, 128, 128),
    (192, 192, 192),
    (128, 128, 128),
    (255, 0, 0),
    (0, 255, 0),
    (255, 255, 0),
    (0, 0, 255),
    (255, 0, 255),
    (0, 255, 255),
    (255, 255, 255),
    (0, 0, 0),
    (0, 0, 95),
    (0, 0, 135),
    (0, 0, 175),
    (0, 0, 215),
    (0, 0, 255),
    (0, 95, 0),
    (0, 95, 95),
    (0, 95, 135),
    (0, 95, 175),
    (0, 95, 215),
    (0, 95, 255),
    (0, 135, 0),
    (0, 135, 95),
    (0, 135, 135),
    (0, 135, 175),
    (0, 135, 215),
    (0, 135, 255),
    (0, 175, 0),
    (0, 175, 95),
    (0, 175, 135),
    (0, 175, 175),
    (0, 175, 215),
    (0, 175, 255),
    (0, 215, 0),
    (0, 215, 95),
    (0, 215, 135),
    (0, 215, 175),
    (0, 215, 215),
    (0, 215, 255),
    (0, 255, 0),
    (0, 255, 95),
    (0, 255, 135),
    (0, 255, 175),
    (0, 255, 215),
    (0, 255, 255),
    (95, 0, 0),
    (95, 0, 95),
    (95, 0, 135),
    (95, 0, 175),
    (95, 0, 215),
    (95, 0, 255),
    (95, 95, 0),
    (95, 95, 95),
    (95, 95, 135),
    (95, 95, 175),
    (95, 95, 215),
    (95, 95, 255),
    (95, 135, 0),
    (95, 135, 95),
    (95, 135, 135),
    (95, 135, 175),
    (95, 135, 215),
    (95, 135, 255),
    (95, 175, 0),
    (95, 175, 95),
    (95, 175, 135),
    (95, 175, 175),
    (95, 175, 215),
    (95, 175, 255),
    (95, 215, 0),
    (95, 215, 95),
    (95, 215, 135),
    (95, 215, 175),
    (95, 215, 215),
    (95, 215, 255),
    (95, 255, 0),
    (95, 255, 95),
    (95, 255, 135),
    (95, 255, 175),
    (95, 255, 215),
    (95, 255, 255),
    (135, 0, 0),
    (135, 0, 95),
    (135, 0, 135),
    (135, 0, 175),
    (135, 0, 215),
    (135, 0, 255),
    (135, 95, 0),
    (135, 95, 95),
    (135, 95, 135),
    (135, 95, 175),
    (135, 95, 215),
    (135, 95, 255),
    (135, 135, 0),
    (135, 135, 95),
    (135, 135, 135),
    (135, 135, 175),
    (135, 135, 215),
    (135, 135, 255),
    (135, 175, 0),
    (135, 175, 95),
    (135, 175, 135),
    (135, 175, 175),
    (135, 175, 215),
    (135, 175, 255),
    (135, 215, 0),
    (135, 215, 95),
    (135, 215, 135),
    (135, 215, 175),
    (135, 215, 215),
    (135, 215, 255),
    (135, 255, 0),
    (135, 255, 95),
    (135, 255, 135),
    (135, 255, 175),
    (135, 255, 215),
    (135, 255, 255),
    (175, 0, 0),
    (175, 0, 95),
    (175, 0, 135),
    (175, 0, 175),
    (175, 0, 215),
    (175, 0, 255),
    (175, 95, 0),
    (175, 95, 95),
    (175, 95, 135),
    (175, 95, 175),
    (175, 95, 215),
    (175, 95, 255),
    (175, 135, 0),
    (175, 135, 95),
    (175, 135, 135),
    (175, 135, 175),
    (175, 135, 215),
    (175, 135, 255),
    (175, 175, 0),
    (175, 175, 95),
    (175, 175, 135),
    (175, 175, 175),
    (175, 175, 215),
    (175, 175, 255),
    (175, 215, 0),
    (175, 215, 95),
    (175, 215, 135),
    (175, 215, 175),
    (175, 215, 215),
    (175, 215, 255),
    (175, 255, 0),
    (175, 255, 95),
    (175, 255, 135),
    (175, 255, 175),
    (175, 255, 215),
    (175, 255, 255),
    (215, 0, 0),
    (215, 0, 95),
    (215, 0, 135),
    (215, 0, 175),
    (215, 0, 215),
    (215, 0, 255),
    (215, 95, 0),
    (215, 95, 95),
    (215, 95, 135),
    (215, 95, 175),
    (215, 95, 215),
    (215, 95, 255),
    (215, 135, 0),
    (215, 135, 95),
    (215, 135, 135),
    (215, 135, 175),
    (215, 135, 215),
    (215, 135, 255),
    (215, 175, 0),
    (215, 175, 95),
    (215, 175, 135),
    (215, 175, 175),
    (215, 175, 215),
    (215, 175, 255),
    (215, 215, 0),
    (215, 215, 95),
    (215, 215, 135),
    (215, 215, 175),
    (215, 215, 215),
    (215, 215, 255),
    (215, 255, 0),
    (215, 255, 95),
    (215, 255, 135),
    (215, 255, 175),
    (215, 255, 215),
    (215, 255, 255),
    (255, 0, 0),
    (255, 0, 95),
    (255, 0, 135),
    (255, 0, 175),
    (255, 0, 215),
    (255, 0, 255),
    (255, 95, 0),
    (255, 95, 95),
    (255, 95, 135),
    (255, 95, 175),
    (255, 95, 215),
    (255, 95, 255),
    (255, 135, 0),
    (255, 135, 95),
    (255, 135, 135),
    (255, 135, 175),
    (255, 135, 215),
    (255, 135, 255),
    (255, 175, 0),
    (255, 175, 95),
    (255, 175, 135),
    (255, 175, 175),
    (255, 175, 215),
    (255, 175, 255),
    (255, 215, 0),
    (255, 215, 95),
    (255, 215, 135),
    (255, 215, 175),
    (255, 215, 215),
    (255, 215, 255),
    (255, 255, 0),
    (255, 255, 95),
    (255, 255, 135),
    (255, 255, 175),
    (255, 255, 215),
    (255, 255, 255),
    (8, 8, 8),
    (18, 18, 18),
    (28, 28, 28),
    (38, 38, 38),
    (48, 48, 48),
    (58, 58, 58),
    (68, 68, 68),
    (78, 78, 78),
    (88, 88, 88),
    (98, 98, 98),
    (108, 108, 108),
    (118, 118, 118),
    (128, 128, 128),
    (138, 138, 138),
    (148, 148, 148),
    (158, 158, 158),
    (168, 168, 168),
    (178, 178, 178),
    (188, 188, 188),
    (198, 198, 198),
    (208, 208, 208),
    (218, 218, 218),
    (228, 228, 228),
    (238, 238, 238),
]
