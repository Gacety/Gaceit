# Source and attribution

The distance solver is adapted from Inigo Quilez’s sdBezier.

// SOURCE: https://www.shadertoy.com/view/MlKcDD
// LICENSE: The MIT License
// Copyright © 2018 Inigo Quilez
// Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the "Software"), to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so, subject to the following conditions: The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software. THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.



The supplied pencil atlas PNG and rectangle metadata originate from the user-provided GoodNotes extraction archive. No redistribution license for those assets was supplied; they are retained here as reference assets for this local implementation. Other engine code is newly organized and implemented from the supplied specification.


## Phosphor Icons — independent native demo toolbar

The toolbar and navigation icons in `cmd/ink-demo` use unmodified official Regular SVG assets from [Phosphor Icons core](https://github.com/phosphor-icons/core), commit `2b75f3ad12b420c9504ef05df8d2564a28f8500e`, under the MIT License. The full upstream copyright and permission notice is retained in [the vendored LICENSE](../../cmd/ink-demo/assets/phosphor/LICENSE); [SOURCE.json](../../cmd/ink-demo/assets/phosphor/SOURCE.json) records each asset's upstream URL and SHA-256. These embedded assets are rendered with native MyGo SVG drawing and do not change the engine or pencil-atlas attribution above.
