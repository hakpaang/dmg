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
    'background': '/Users/leaanthony/Downloads/go-dmg/test-dmgs/test-bg.png',
}

dmgbuild.build_dmg(
    filename='/Users/leaanthony/Downloads/go-dmg/test-dmgs/golden/image-bg.dmg',
    volume_name='Image BG Test',
    settings=settings,
)

print("Generated: image-bg.dmg")
