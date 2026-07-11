#!/usr/bin/env python3
"""Бинарно патчит захардкоженный путь webkit2gtk-4.1 в файлах AppDir на /tmp/.mise-webkit,
чтобы webkit нашёл свои процессы внутри AppImage (WEBKIT_EXEC_PATH этой версией не поддерживается)."""
import os, sys

root = sys.argv[1] if len(sys.argv) > 1 else "AppDir"
old2 = b'/usr/lib/x86_64-linux-gnu/webkit2gtk-4.1/injected-bundle/'
new2 = b'/tmp/.mise-webkit/injected-bundle/'
old1 = b'/usr/lib/x86_64-linux-gnu/webkit2gtk-4.1'
new1 = b'/tmp/.mise-webkit'

n = 0
for dp, _, fs in os.walk(root):
    for f in fs:
        p = os.path.join(dp, f)
        try:
            d = open(p, 'rb').read()
        except Exception:
            continue
        if old1 not in d:
            continue
        d = d.replace(old2, new2 + b'\x00' * (len(old2) - len(new2)))
        d = d.replace(old1, new1 + b'\x00' * (len(old1) - len(new1)))
        open(p, 'wb').write(d)
        n += 1
        print("patched:", p)
print("webkit path patched in", n, "files")
