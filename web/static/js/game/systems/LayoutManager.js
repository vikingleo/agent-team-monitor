export class LayoutManager {
    constructor(scene) {
        this.scene = scene;
        this.facilities = [];
    }

    calculateLayout(teams) {
        const teamCount = teams.length;

        // 使用浏览器窗口尺寸，减去侧栏宽度
        const viewWidth = window.innerWidth - 490;  // 减去侧栏 (400px) + tab (90px)
        const viewHeight = window.innerHeight;

        const layout = {
            teams: [],
            facilities: [],
            zones: [],
            bounds: { width: viewWidth, height: viewHeight }
        };

        if (teamCount === 0) return layout;

        // 将团队按类型分组（Claude 和 Codex）
        const claudeTeams = teams.filter(t => this.isClaudeTeam(t));
        const codexTeams = teams.filter(t => !this.isClaudeTeam(t));

        // 开放式办公区布局 - 根据窗口大小调整
        if (teamCount <= 3) {
            layout.facilities = this.getSmallOfficeFacilities(viewWidth, viewHeight);
            layout.zones = this.getSmallOfficeZones(viewWidth, viewHeight);
            layout.teams = this.layoutSmallOffice(claudeTeams, codexTeams, viewWidth, viewHeight);
        }
        else if (teamCount <= 6) {
            layout.facilities = this.getMediumOfficeFacilities(viewWidth, viewHeight);
            layout.zones = this.getMediumOfficeZones(viewWidth, viewHeight);
            layout.teams = this.layoutMediumOffice(claudeTeams, codexTeams, viewWidth, viewHeight);
        }
        else {
            layout.facilities = this.getLargeOfficeFacilities(viewWidth, viewHeight);
            layout.zones = this.getLargeOfficeZones(viewWidth, viewHeight);
            layout.teams = this.layoutLargeOffice(claudeTeams, codexTeams, viewWidth, viewHeight);
        }

        return layout;
    }

    // 判断是否为 Claude 团队（根据团队名称或其他特征）
    isClaudeTeam(team) {
        const name = team.name.toLowerCase();
        return name.includes('claude') || name.includes('anthropic');
    }

    layoutSmallOffice(claudeTeams, codexTeams, viewWidth, viewHeight) {
        const positions = [];
        const halfWidth = viewWidth / 2;

        // Claude 区域（左侧）
        claudeTeams.forEach((team, i) => {
            positions.push({
                name: team.name,
                x: halfWidth * 0.5,
                y: viewHeight * 0.3 + i * 200,
                rotation: 0,
                isDesk: true,
                zone: 'claude'
            });
        });

        // Codex 区域（右侧）
        codexTeams.forEach((team, i) => {
            positions.push({
                name: team.name,
                x: halfWidth * 1.5,
                y: viewHeight * 0.3 + i * 200,
                rotation: 0,
                isDesk: true,
                zone: 'codex'
            });
        });

        return positions;
    }

    layoutMediumOffice(claudeTeams, codexTeams, viewWidth, viewHeight) {
        const positions = [];
        const halfWidth = viewWidth / 2;

        // Claude 区域（左半部分）
        claudeTeams.forEach((team, i) => {
            const row = Math.floor(i / 2);
            const col = i % 2;
            positions.push({
                name: team.name,
                x: halfWidth * 0.3 + col * 300,
                y: viewHeight * 0.25 + row * 250,
                rotation: (Math.random() - 0.5) * 10,
                isDesk: true,
                zone: 'claude'
            });
        });

        // Codex 区域（右半部分）
        codexTeams.forEach((team, i) => {
            const row = Math.floor(i / 2);
            const col = i % 2;
            positions.push({
                name: team.name,
                x: halfWidth * 1.2 + col * 300,
                y: viewHeight * 0.25 + row * 250,
                rotation: (Math.random() - 0.5) * 10,
                isDesk: true,
                zone: 'codex'
            });
        });

        return positions;
    }

    layoutLargeOffice(claudeTeams, codexTeams, viewWidth, viewHeight) {
        const positions = [];
        const halfWidth = viewWidth / 2;

        // Claude 区域（左半部分）
        claudeTeams.forEach((team, i) => {
            const row = Math.floor(i / 2);
            const col = i % 2;
            positions.push({
                name: team.name,
                x: halfWidth * 0.3 + col * 350,
                y: viewHeight * 0.2 + row * 280,
                rotation: (Math.random() - 0.5) * 15,
                isDesk: true,
                zone: 'claude'
            });
        });

        // Codex 区域（右半部分）
        codexTeams.forEach((team, i) => {
            const row = Math.floor(i / 2);
            const col = i % 2;
            positions.push({
                name: team.name,
                x: halfWidth * 1.3 + col * 350,
                y: viewHeight * 0.2 + row * 280,
                rotation: (Math.random() - 0.5) * 15,
                isDesk: true,
                zone: 'codex'
            });
        });

        return positions;
    }

    getSmallOfficeZones(viewWidth, viewHeight) {
        const halfWidth = viewWidth / 2;
        return [
            { name: 'Claude 区', x: 50, y: 50, width: halfWidth - 100, height: viewHeight - 100, color: 0xE8F4F8 },
            { name: 'Codex 区', x: halfWidth + 50, y: 50, width: halfWidth - 100, height: viewHeight - 100, color: 0xFFF4E6 }
        ];
    }

    getMediumOfficeZones(viewWidth, viewHeight) {
        const halfWidth = viewWidth / 2;
        return [
            { name: 'Claude 区', x: 50, y: 50, width: halfWidth - 100, height: viewHeight - 100, color: 0xE8F4F8 },
            { name: 'Codex 区', x: halfWidth + 50, y: 50, width: halfWidth - 100, height: viewHeight - 100, color: 0xFFF4E6 }
        ];
    }

    getLargeOfficeZones(viewWidth, viewHeight) {
        const halfWidth = viewWidth / 2;
        return [
            { name: 'Claude 区', x: 50, y: 50, width: halfWidth - 100, height: viewHeight - 100, color: 0xE8F4F8 },
            { name: 'Codex 区', x: halfWidth + 50, y: 50, width: halfWidth - 100, height: viewHeight - 100, color: 0xFFF4E6 }
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
