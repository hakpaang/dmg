#!/Users/leaanthony/Downloads/dmg-upstream/.venv/bin/python3
"""Window positioned at top-left corner - tests window position handling."""

import dmgbuild

settings = {
    'format': 'UDBZ',
    'size': None,
    'files': [('/Users/leaanthony/Downloads/dmg-upstream/test-dmgs/MyApp.app', 'MyApp.app')],
    'symlinks': {'Applications': '/Applications'},
    'icon_size': 128,
    'text_size': 12,
    'icon_locations': {
        'MyApp.app': (180, 170),
        'Applications': (360, 170),
    },
    'window_rect': ((50, 50), (540, 380)),
    'background': '/Users/leaanthony/Downloads/dmg-upstream/test-dmgs/test-bg.png',
}

dmgbuild.build_dmg(
    filename='/Users/leaanthony/Downloads/dmg-upstream/test-dmgs/golden/top-left.dmg',
    volume_name='Top Left Test',
    settings=settings,
)

print("Generated: top-left.dmg")
