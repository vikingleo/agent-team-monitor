import { GameConfig } from './config.js';
import { Sidebar } from '../components/Sidebar.js';

class Game {
    constructor() {
        this.game = null;
        this.sidebar = null;
    }

    init() {
        this.game = new Phaser.Game(GameConfig);
        this.sidebar = new Sidebar();
        console.log('Phaser game initialized');

        // 监听场景状态更新
        this.game.events.on('state-updated', (state) => {
            if (this.sidebar) {
                this.sidebar.updateState(state);
            }
        });
    }

    destroy() {
        if (this.sidebar) {
            this.sidebar.destroy();
        }
        if (this.game) {
            this.game.destroy(true);
        }
    }
}

// 全局实例
window.AgentMonitorGame = new Game();

// 页面加载后初始化
document.addEventListener('DOMContentLoaded', () => {
    window.AgentMonitorGame.init();
});
