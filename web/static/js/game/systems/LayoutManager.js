export class LayoutManager {
    constructor(scene) {
        this.scene = scene;
        this.facilities = [];
    }

    calculateLayout(teams) {
        const teamCount = teams.length;
        const layout = {
            teams: [],
            facilities: [],
            bounds: { width: 800, height: 600 }
        };

        if (teamCount === 0) return layout;

        // 小公司布局
        if (teamCount <= 2) {
            layout.bounds = { width: 800, height: 600 };
            layout.facilities = this.getSmallCompanyFacilities();
            layout.teams = this.layoutSmallCompany(teams);
        }
        // 中型公司
        else if (teamCount <= 5) {
            layout.bounds = { width: 1200, height: 800 };
            layout.facilities = this.getMediumCompanyFacilities();
            layout.teams = this.layoutMediumCompany(teams);
        }
        // 大型公司
        else {
            const rows = Math.ceil(teamCount / 3);
            layout.bounds = { width: 1600, height: 400 + rows * 350 };
            layout.facilities = this.getLargeCompanyFacilities();
            layout.teams = this.layoutLargeCompany(teams);
        }

        return layout;
    }

    layoutSmallCompany(teams) {
        const positions = [
            { x: 150, y: 100 },
            { x: 150, y: 400 }
        ];
        return teams.map((team, i) => ({
            name: team.name,
            x: positions[i].x,
            y: positions[i].y
        }));
    }

    layoutMediumCompany(teams) {
        const positions = [
            { x: 150, y: 100 },
            { x: 500, y: 100 },
            { x: 150, y: 500 },
            { x: 500, y: 500 },
            { x: 850, y: 500 }
        ];
        return teams.map((team, i) => ({
            name: team.name,
            x: positions[i].x,
            y: positions[i].y
        }));
    }

    layoutLargeCompany(teams) {
        const positions = [];
        const cols = 3;
        const startX = 200;
        const startY = 200;
        const spacingX = 450;  // 增加横向间距
        const spacingY = 350;  // 增加纵向间距

        teams.forEach((team, i) => {
            const col = i % cols;
            const row = Math.floor(i / cols);
            positions.push({
                name: team.name,
                x: startX + col * spacingX,
                y: startY + row * spacingY
            });
        });

        return positions;
    }

    getSmallCompanyFacilities() {
        return [
            { type: 'restroom', x: 50, y: 50, width: 80, height: 100 },
            { type: 'cafe', x: 50, y: 450, width: 80, height: 100 }
        ];
    }

    getMediumCompanyFacilities() {
        return [
            { type: 'restroom', x: 50, y: 50, width: 80, height: 100 },
            { type: 'gym', x: 900, y: 50, width: 150, height: 100 },
            { type: 'cafe', x: 50, y: 650, width: 80, height: 100 },
            { type: 'boss', x: 550, y: 350, width: 180, height: 120 }
        ];
    }

    getLargeCompanyFacilities() {
        return [
            { type: 'restroom', x: 50, y: 50, width: 100, height: 120 },
            { type: 'gym', x: 1400, y: 50, width: 150, height: 120 },
            { type: 'cafe', x: 50, y: 900, width: 100, height: 120 },
            { type: 'boss', x: 700, y: 50, width: 200, height: 120 }
        ];
    }
}
