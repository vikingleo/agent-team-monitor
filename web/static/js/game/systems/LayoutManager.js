const PROVIDER_ZONE_LABELS = {
    claude: 'Claude Code',
    codex: 'Codex',
    openclaw: 'OpenClaw',
    unknown: 'Mixed'
};

export class LayoutManager {
    constructor(scene) {
        this.scene = scene;
        this.facilities = [];
    }

    calculateLayout(teams) {
        const teamCount = teams.length;
        const sidebarWidth = this.getSidebarWidth();
        const viewWidth = Math.max(window.innerWidth - sidebarWidth, 320);
        const viewHeight = window.innerHeight;

        const layout = {
            teams: [],
            facilities: [],
            zones: [],
            bounds: { width: viewWidth, height: viewHeight }
        };

        if (teamCount === 0) {
            return layout;
        }

        if (teamCount <= 3) {
            layout.facilities = this.getSmallOfficeFacilities(viewWidth, viewHeight);
        }
        else if (teamCount <= 6) {
            layout.facilities = this.getMediumOfficeFacilities(viewWidth, viewHeight);
        }
        else {
            layout.facilities = this.getLargeOfficeFacilities(viewWidth, viewHeight);
        }

        const sharedOffice = this.layoutSharedOffice(teams, viewWidth, viewHeight, layout.facilities);
        layout.teams = sharedOffice.teams;
        layout.zones = sharedOffice.zones;
        layout.bounds = sharedOffice.bounds;
        return layout;
    }

    getSidebarWidth() {
        const rootStyles = window.getComputedStyle(document.body);
        const cssWidth = parseFloat(rootStyles.getPropertyValue('--game-sidebar-width'));
        if (Number.isFinite(cssWidth) && cssWidth > 0) {
            return cssWidth;
        }
        return window.innerWidth * 0.33;
    }

    detectTeamProvider(team) {
        const direct = String((team && team.provider) || '').toLowerCase();
        if (direct === 'claude' || direct === 'codex' || direct === 'openclaw') {
            return direct;
        }

        const members = Array.isArray(team?.members) ? team.members : [];
        const memberProvider = members
            .map((member) => String(member?.provider || '').toLowerCase())
            .find((provider) => provider === 'claude' || provider === 'codex' || provider === 'openclaw');
        if (memberProvider) {
            return memberProvider;
        }

        const name = String((team && team.name) || '').toLowerCase();
        if (name.includes('claude') || name.includes('anthropic')) {
            return 'claude';
        }
        if (name.includes('openclaw')) {
            return 'openclaw';
        }
        if (name.includes('codex')) {
            return 'codex';
        }

        return 'unknown';
    }

    layoutSharedOffice(teams, viewWidth, viewHeight, facilities) {
        const facilityFrames = facilities.map((facility) => ({
            left: facility.x - 72,
            top: facility.y - 84,
            right: facility.x + facility.width + 72,
            bottom: facility.y + facility.height + 84
        }));

        const positions = [];
        const baseDeskWidth = Math.max(250, ...teams.map((team) => this.getDeskWidth(team)));
        const leftMargin = 96;
        const rightMargin = 96;
        const topMargin = 124;
        const bottomMargin = 140;
        const columnGap = 84;
        const rowGap = 148;
        const corridorHeight = 280;
        const columnWidth = baseDeskWidth + columnGap;
        const rowHeight = 92 + rowGap;
        const usableWidth = Math.max(viewWidth - leftMargin - rightMargin, baseDeskWidth);
        const columnCount = Math.max(1, Math.floor((usableWidth + columnGap) / columnWidth));
        const layerCount = Math.max(1, Math.ceil(teams.length / (columnCount * 2)));
        const slotCandidates = [];

        for (let layer = 0; layer < layerCount + 2; layer += 1) {
            const topY = topMargin + Math.max(0, layerCount - 1 - layer) * rowHeight;
            const bottomY = topMargin + layerCount * rowHeight + corridorHeight + layer * rowHeight;
            for (let column = 0; column < columnCount; column += 1) {
                const x = leftMargin + baseDeskWidth / 2 + column * columnWidth;
                slotCandidates.push({ x, y: topY });
                slotCandidates.push({ x, y: bottomY });
            }
        }

        let candidateIndex = 0;

        teams.forEach((team) => {
            const provider = this.detectTeamProvider(team);
            const deskWidth = this.getDeskWidth(team);

            while (true) {
                const slot = slotCandidates[candidateIndex];
                if (!slot) {
                    const overflowLayer = Math.floor(candidateIndex / (columnCount * 2));
                    const overflowColumn = Math.floor((candidateIndex % (columnCount * 2)) / 2);
                    const overflowBottom = candidateIndex % 2 === 1;
                    const x = leftMargin + baseDeskWidth / 2 + overflowColumn * columnWidth;
                    const y = overflowBottom
                        ? topMargin + layerCount * rowHeight + corridorHeight + overflowLayer * rowHeight
                        : topMargin + Math.max(0, layerCount - 1 - overflowLayer) * rowHeight;
                    slotCandidates.push({ x, y });
                    continue;
                }

                candidateIndex += 1;
                const { x, y } = slot;
                const frame = {
                    left: x - deskWidth / 2 - 34,
                    right: x + deskWidth / 2 + 34,
                    top: y - 76,
                    bottom: y + 148
                };

                if (!this.intersectsAny(frame, facilityFrames)) {
                    positions.push({
                        name: team.name,
                        x,
                        y,
                        rotation: 0,
                        isDesk: true,
                        zone: provider,
                        provider,
                        zoneLabel: PROVIDER_ZONE_LABELS[provider] || PROVIDER_ZONE_LABELS.unknown,
                        deskWidth
                    });
                    break;
                }
            }
        });

        const maxBottom = positions.length > 0 ? Math.max(...positions.map((item) => item.y + 148)) : viewHeight - bottomMargin;
        const contentHeight = Math.max(
            viewHeight,
            topMargin + layerCount * rowHeight + corridorHeight + bottomMargin,
            maxBottom + bottomMargin
        );
        const contentWidth = Math.max(viewWidth, leftMargin + columnCount * columnWidth + rightMargin - columnGap);
        const corridorY = topMargin + layerCount * rowHeight + 18;
        const corridorHeightInner = corridorHeight - 36;
        const entryWidth = Math.max(96, Math.min(148, contentWidth * 0.11));

        return {
            teams: positions,
            zones: [
                {
                    name: 'Main Corridor',
                    kind: 'corridor',
                    x: 52,
                    y: corridorY,
                    width: contentWidth - 104,
                    height: corridorHeightInner,
                    color: 0xd8dee6
                },
                {
                    name: 'West Entry',
                    kind: 'entry',
                    side: 'left',
                    x: 52,
                    y: corridorY + 20,
                    width: entryWidth,
                    height: corridorHeightInner - 40,
                    color: 0xe5e7eb
                },
                {
                    name: 'East Entry',
                    kind: 'entry',
                    side: 'right',
                    x: contentWidth - 52 - entryWidth,
                    y: corridorY + 20,
                    width: entryWidth,
                    height: corridorHeightInner - 40,
                    color: 0xe5e7eb
                }
            ],
            bounds: {
                width: contentWidth,
                height: Math.max(viewHeight, contentHeight)
            }
        };
    }

    intersectsAny(frame, blockedFrames) {
        return blockedFrames.some((blocked) => !(
            frame.right < blocked.left ||
            frame.left > blocked.right ||
            frame.bottom < blocked.top ||
            frame.top > blocked.bottom
        ));
    }

    getDeskWidth(team) {
        const members = Array.isArray(team?.members) ? team.members : [];
        return Math.max(200, members.length * 60 + 40);
    }

    getSmallOfficeFacilities(viewWidth, viewHeight) {
        return [
            { type: 'restroom', x: 44, y: 56, width: 96, height: 76 },
            { type: 'cafe', x: viewWidth - 174, y: 60, width: 118, height: 92 },
            { type: 'boss', x: viewWidth - 228, y: viewHeight - 172, width: 172, height: 104 }
        ];
    }

    getMediumOfficeFacilities(viewWidth, viewHeight) {
        return [
            { type: 'restroom', x: 44, y: 60, width: 104, height: 84 },
            { type: 'cafe', x: viewWidth - 216, y: 64, width: 140, height: 108 },
            { type: 'gym', x: 56, y: viewHeight - 194, width: 142, height: 112 },
            { type: 'boss', x: viewWidth - 250, y: viewHeight - 210, width: 194, height: 116 }
        ];
    }

    getLargeOfficeFacilities(viewWidth, viewHeight) {
        return [
            { type: 'restroom', x: 48, y: 64, width: 112, height: 92 },
            { type: 'restroom', x: viewWidth - 176, y: 64, width: 112, height: 92 },
            { type: 'cafe', x: viewWidth - 250, y: viewHeight - 236, width: 170, height: 132 },
            { type: 'gym', x: 56, y: viewHeight - 230, width: 176, height: 132 },
            { type: 'boss', x: viewWidth / 2 - 112, y: 44, width: 224, height: 118 }
        ];
    }
}
