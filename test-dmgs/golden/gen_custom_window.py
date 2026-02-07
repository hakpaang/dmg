#!/Users/leaanthony/Downloads/go-dmg/.venv/bin/python3

import dmgbuild
import os

# Settings dict that dmgbuild accepts
settings = {
    'format': 'UDBZ',
    'size': None,  # auto-calculate
    'files': [('/Users/leaanthony/Downloads/go-dmg/test-dmgs/MyApp.app', 'MyApp.app')],
    'symlinks': {'Applications': '/Applications'},
    'icon_size': 96,
    'text_size': 14,
    'icon_locations': {
        'MyApp.app': (200, 180),
        'Applications': (400, 180),
    },
    'window_rect': ((100, 100), (600, 400)),  # ((x, y), (w, h))
    'background': '/Users/leaanthony/Downloads/go-dmg/test-dmgs/test-bg.png',
}

dmgbuild.build_dmg(
    filename='/Users/leaanthony/Downloads/go-dmg/test-dmgs/golden/custom-window.dmg',
    volume_name='Custom Window Test',
    settings=settings,
)

print("Generated: custom-window.dmg")
