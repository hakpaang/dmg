#!/Users/leaanthony/Downloads/dmg-upstream/.venv/bin/python3
"""Large window with image background - tests non-standard window dimensions."""

import dmgbuild

settings = {
    'format': 'UDBZ',
    'size': None,
    'files': [('/Users/leaanthony/Downloads/dmg-upstream/test-dmgs/MyApp.app', 'MyApp.app')],
    'symlinks': {'Applications': '/Applications'},
    'icon_size': 160,
    'text_size': 14,
    'icon_locations': {
        'MyApp.app': (250, 250),
        'Applications': (550, 250),
    },
    'window_rect': ((100, 100), (800, 500)),
    'background': '/Users/leaanthony/Downloads/dmg-upstream/test-dmgs/test-bg.png',
}

dmgbuild.build_dmg(
    filename='/Users/leaanthony/Downloads/dmg-upstream/test-dmgs/golden/large-window.dmg',
    volume_name='Large Window Test',
    settings=settings,
)

print("Generated: large-window.dmg")
