# hostile

HTML blocks hide ESM from a paragraph-only check.

<div>
import fs from "node:fs"
</div>

<p>
export default function X(){}
</p>

<details>
export FOO=bar
</details>

<details>
<summary>Env</summary>

export FOO=bar

</details>

<pre>
first

import os from "node:os"
</pre>

<b>import</b> starts this paragraph after an inline tag.

A "quoted\r\rtag" is <i>fine</i>.

[imp](javascript:alert(1))ort x from "y"

[exp](javascript:x)ort const a = 1

![imp](data:text/html,x)ort x from "y"

See [one][r1], [two][r2], [three][r3], and [four][r4].

[r1]: javascript&amp;colon;alert(1)
[r2]: javascript&#58;alert(2)
[r3]: javascript&#x3a;alert(3)
[r4]: <javascript&colon;alert(4)>
