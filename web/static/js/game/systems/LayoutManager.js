export class LayoutManager {
    constructor(scene) {
        this.scene = scene;
        this.facilities = [];
    }

    calculateLayout(teams) {
        const teamCount = teams.length;

        // 使用浏览器窗口尺寸，减去侧栏宽度
        const sidebarWidth = this.getSidebarWidth();
        const viewWidth = Math.max(window.innerWidth - sidebarWidth, 320);
        const viewHeight = window.innerHeight;

        const layout = {
            teams: [],
            facilities: [],
            zones: [],
            bounds: { width: viewWidth, height: viewHeight }
        };

        if (teamCount === 0) return layout;

        // 将团队按类型分组（Claude / OpenClaw / 其他）
        const claudeTeams = teams.filter(t => this.detectTeamProvider(t) === 'claude');
        const openClawTeams = teams.filter(t => this.detectTeamProvider(t) === 'openclaw');
        const codexTeams = teams.filter(t => {
            const provider = this.detectTeamProvider(t);
            return provider !== 'claude' && provider !== 'openclaw';
        });

        // 开放式办公区布局 - 根据窗口大小调整
        if (teamCount <= 3) {
            layout.facilities = this.getSmallOfficeFacilities(viewWidth, viewHeight);
            layout.zones = this.getSmallOfficeZones(viewWidth, viewHeight);
            layout.teams = this.layoutSmallOffice(claudeTeams, codexTeams, openClawTeams, viewWidth, viewHeight);
        }
        else if (teamCount <= 6) {
            layout.facilities = this.getMediumOfficeFacilities(viewWidth, viewHeight);
            layout.zones = this.getMediumOfficeZones(viewWidth, viewHeight);
            layout.teams = this.layoutMediumOffice(claudeTeams, codexTeams, openClawTeams, viewWidth, viewHeight);
        }
        else {
            layout.facilities = this.getLargeOfficeFacilities(viewWidth, viewHeight);
            layout.zones = this.getLargeOfficeZones(viewWidth, viewHeight);
            layout.teams = this.layoutLargeOffice(claudeTeams, codexTeams, openClawTeams, viewWidth, viewHeight);
        }

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

    // 判断是否为 Claude 团队（根据团队名称或其他特征）
    isClaudeTeam(team) {
        return this.detectTeamProvider(team) === 'claude';
    }

    detectTeamProvider(team) {
        const direct = String((team && team.provider) || '').toLowerCase();
        if (direct === 'claude' || direct === 'codex' || direct === 'openclaw') {
            return direct;
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

    layoutSmallOffice(claudeTeams, codexTeams, openClawTeams, viewWidth, viewHeight) {
        const positions = [];
        const zoneWidth = viewWidth / 3;

        claudeTeams.forEach((team, i) => {
            positions.push({
                name: team.name,
                x: zoneWidth * 0.5,
                y: viewHeight * 0.3 + i * 200,
                rotation: 0,
                isDesk: true,
                zone: 'claude'
            });
        });

        codexTeams.forEach((team, i) => {
            positions.push({
                name: team.name,
                x: zoneWidth * 1.5,
                y: viewHeight * 0.3 + i * 200,
                rotation: 0,
                isDesk: true,
                zone: 'codex'
            });
        });

        openClawTeams.forEach((team, i) => {
            positions.push({
                name: team.name,
                x: zoneWidth * 2.5,
                y: viewHeight * 0.3 + i * 200,
                rotation: 0,
                isDesk: true,
                zone: 'openclaw'
            });
        });

        return positions;
    }

    layoutMediumOffice(claudeTeams, codexTeams, openClawTeams, viewWidth, viewHeight) {
        const positions = [];
        const zoneWidth = viewWidth / 3;

        claudeTeams.forEach((team, i) => {
            const row = Math.floor(i / 2);
            const col = i % 2;
            positions.push({
                name: team.name,
                x: zoneWidth * 0.35 + col * 120,
                y: viewHeight * 0.25 + row * 250,
                rotation: this.getTeamRotation(team.name, 10),
                isDesk: true,
                zone: 'claude'
            });
        });

        codexTeams.forEach((team, i) => {
            const row = Math.floor(i / 2);
            const col = i % 2;
            positions.push({
                name: team.name,
                x: zoneWidth * 1.35 + col * 120,
                y: viewHeight * 0.25 + row * 250,
                rotation: this.getTeamRotation(team.name, 10),
                isDesk: true,
                zone: 'codex'
            });
        });

        openClawTeams.forEach((team, i) => {
            const row = Math.floor(i / 2);
            const col = i % 2;
            positions.push({
                name: team.name,
                x: zoneWidth * 2.35 + col * 120,
                y: viewHeight * 0.25 + row * 250,
                rotation: this.getTeamRotation(team.name, 10),
                isDesk: true,
                zone: 'openclaw'
            });
        });

        return positions;
    }

    layoutLargeOffice(claudeTeams, codexTeams, openClawTeams, viewWidth, viewHeight) {
        const positions = [];
        const zoneWidth = viewWidth / 3;

        claudeTeams.forEach((team, i) => {
            const row = Math.floor(i / 2);
            const col = i % 2;
            positions.push({
                name: team.name,
                x: zoneWidth * 0.35 + col * 130,
                y: viewHeight * 0.2 + row * 280,
                rotation: this.getTeamRotation(team.name, 15),
                isDesk: true,
                zone: 'claude'
            });
        });

        codexTeams.forEach((team, i) => {
            const row = Math.floor(i / 2);
            const col = i % 2;
            positions.push({
                name: team.name,
                x: zoneWidth * 1.35 + col * 130,
                y: viewHeight * 0.2 + row * 280,
                rotation: this.getTeamRotation(team.name, 15),
                isDesk: true,
                zone: 'codex'
            });
        });

        openClawTeams.forEach((team, i) => {
            const row = Math.floor(i / 2);
            const col = i % 2;
            positions.push({
                name: team.name,
                x: zoneWidth * 2.35 + col * 130,
                y: viewHeight * 0.2 + row * 280,
                rotation: this.getTeamRotation(team.name, 15),
                isDesk: true,
                zone: 'openclaw'
            });
        });

        return positions;
    }

    getTeamRotation(teamName, maxDegrees) {
        const hash = this.hashString(teamName);
        const normalized = (hash % 1000) / 999;
        return (normalized - 0.5) * maxDegrees;
    }

    hashString(value) {
        let hash = 0;
        for (let index = 0; index < value.length; index += 1) {
            hash = ((hash << 5) - hash) + value.charCodeAt(index);
            hash |= 0;
        }
        return Math.abs(hash);
    }

    getSmallOfficeZones(viewWidth, viewHeight) {
        const zoneWidth = viewWidth / 3;
        return [
            { name: 'Claude 区', x: 30, y: 50, width: zoneWidth - 60, height: viewHeight - 100, color: 0xE8F4F8 },
            { name: 'Codex 区', x: zoneWidth + 30, y: 50, width: zoneWidth - 60, height: viewHeight - 100, color: 0xFFF4E6 },
            { name: 'OpenClaw 区', x: zoneWidth * 2 + 30, y: 50, width: zoneWidth - 60, height: viewHeight - 100, color: 0xEAF8EA }
        ];
    }

    getMediumOfficeZones(viewWidth, viewHeight) {
        const zoneWidth = viewWidth / 3;
        return [
            { name: 'Claude 区', x: 30, y: 50, width: zoneWidth - 60, height: viewHeight - 100, color: 0xE8F4F8 },
            { name: 'Codex 区', x: zoneWidth + 30, y: 50, width: zoneWidth - 60, height: viewHeight - 100, color: 0xFFF4E6 },
            { name: 'OpenClaw 区', x: zoneWidth * 2 + 30, y: 50, width: zoneWidth - 60, height: viewHeight - 100, color: 0xEAF8EA }
        ];
    }

    getLargeOfficeZones(viewWidth, viewHeight) {
        const zoneWidth = viewWidth / 3;
        return [
            { name: 'Claude 区', x: 30, y: 50, width: zoneWidth - 60, height: viewHeight - 100, color: 0xE8F4F8 },
            { name: 'Codex 区', x: zoneWidth + 30, y: 50, width: zoneWidth - 60, height: viewHeight - 100, color: 0xFFF4E6 },
            { name: 'OpenClaw 区', x: zoneWidth * 2 + 30, y: 50, width: zoneWidth - 60, height: viewHeight - 100, color: 0xEAF8EA }
        ];
    }

    getSmallOfficeFacilities(viewWidth, viewHeight) {
        return [
            { type: 'restroom', x: 80, y: 100, width: 100, height: 80 },
            { type: 'cafe', x: viewWidth - 200, y: viewHeight - 150, width: 120, height: 100 }
        ];
    }

    getMediumOfficeFacilities(viewWidth, viewHeight) {
        return [
            { type: 'restroom', x: 80, y: 120, width: 110, height: 90 },
            { type: 'cafe', x: viewWidth - 220, y: 150, width: 150, height: 120 },
            { type: 'gym', x: 100, y: viewHeight - 180, width: 140, height: 110 },
            { type: 'boss', x: viewWidth - 250, y: viewHeight - 200, width: 180, height: 140 }
        ];
    }

    getLargeOfficeFacilities(viewWidth, viewHeight) {
        return [
            { type: 'restroom', x: 100, y: 150, width: 120, height: 100 },
            { type: 'restroom', x: viewWidth - 200, y: viewHeight - 180, width: 120, height: 100 },
            { type: 'cafe', x: viewWidth - 250, y: 200, width: 180, height: 150 },
            { type: 'gym', x: 150, y: viewHeight - 220, width: 200, height: 150 },
            { type: 'boss', x: viewWidth / 2 - 100, y: 80, width: 200, height: 120 }
        ];
    }
}
