#!/Users/leaanthony/Downloads/go-dmg/.venv/bin/python3

import dmgbuild
import os

# Settings dict that dmgbuild accepts
settings = {
    'format': 'UDBZ',
    'size': None,  # auto-calculate
    'files': [('/Users/leaanthony/Downloads/go-dmg/test-dmgs/MyApp.app', 'MyApp.app')],
    'symlinks': {'Applications': '/Applications'},
    'icon_size': 128,
    'text_size': 12,
    'icon_locations': {
        'MyApp.app': (180, 170),
        'Applications': (360, 170),
    },
    'window_rect': ((200, 200), (540, 380)),  # ((x, y), (w, h))
    'background_color': '#FF0000',  # solid red background
}

dmgbuild.build_dmg(
    filename='/Users/leaanthony/Downloads/go-dmg/test-dmgs/golden/color-bg.dmg',
    volume_name='Color BG Test',
    settings=settings,
)

print("Generated: color-bg.dmg")
