#!/Users/leaanthony/Downloads/dmg-upstream/.venv/bin/python3

import dmgbuild

settings = {
    'format': 'UDBZ',
    'size': None,
    'files': [('/Users/leaanthony/Downloads/dmg-upstream/test-dmgs/MyApp.app', 'MyApp.app')],
    'symlinks': {'Applications': '/Applications'},
    'icon_size': 96,
    'text_size': 14,
    'icon_locations': {
        'MyApp.app': (200, 180),
        'Applications': (400, 180),
    },
    'window_rect': ((100, 100), (600, 400)),
    'background': '/Users/leaanthony/Downloads/dmg-upstream/test-dmgs/test-bg.png',
}

dmgbuild.build_dmg(
    filename='/Users/leaanthony/Downloads/dmg-upstream/test-dmgs/golden/custom-window.dmg',
    volume_name='Custom Window Test',
    settings=settings,
)

print("Generated: custom-window.dmg")
