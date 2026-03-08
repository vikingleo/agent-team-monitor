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
            layout.bounds = { width: 1400 + Math.floor((teamCount - 5) / 2) * 300, height: 1000 };
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
        const startX = 150;
        const startY = 100;
        const spacingX = 350;
        const spacingY = 300;

        teams.forEach((team, i) => {
            const col = i % cols;
            const row = Math.floor(i / cols);
            positions.push({
                name: team.name,
                x: startX + col * spacingX + (Math.random() - 0.5) * 40,
                y: startY + row * spacingY + (Math.random() - 0.5) * 40
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
            { type: 'restroom', x: 50, y: 50, width: 80, height: 100 },
            { type: 'gym', x: 1100, y: 50, width: 150, height: 100 },
            { type: 'cafe', x: 50, y: 850, width: 80, height: 100 },
            { type: 'boss', x: 650, y: 450, width: 200, height: 150 }
        ];
    }
}
