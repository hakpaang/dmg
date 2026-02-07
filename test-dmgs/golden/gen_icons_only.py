#!/Users/leaanthony/Downloads/dmg-upstream/.venv/bin/python3

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
    'window_rect': ((200, 200), (540, 380)),
    'background': None,
}

dmgbuild.build_dmg(
    filename='/Users/leaanthony/Downloads/dmg-upstream/test-dmgs/golden/icons-only.dmg',
    volume_name='Icons Only Test',
    settings=settings,
)

print("Generated: icons-only.dmg")
