#!/Users/leaanthony/Downloads/dmg-upstream/.venv/bin/python3
"""Small icons with large text - tests icon/text size variations."""

import dmgbuild

settings = {
    'format': 'UDBZ',
    'size': None,
    'files': [('/Users/leaanthony/Downloads/dmg-upstream/test-dmgs/MyApp.app', 'MyApp.app')],
    'symlinks': {'Applications': '/Applications'},
    'icon_size': 64,
    'text_size': 16,
    'icon_locations': {
        'MyApp.app': (180, 170),
        'Applications': (360, 170),
    },
    'window_rect': ((200, 200), (540, 380)),
    'background_color': '#336699',
}

dmgbuild.build_dmg(
    filename='/Users/leaanthony/Downloads/dmg-upstream/test-dmgs/golden/small-icons.dmg',
    volume_name='Small Icons Test',
    settings=settings,
)

print("Generated: small-icons.dmg")
