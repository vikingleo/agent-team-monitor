import { Sidebar } from '../components/Sidebar.js';
import { CanvasOfficeScene } from './scenes/CanvasOfficeScene.js';

class Game {
    constructor() {
        this.canvas = null;
        this.scene = null;
        this.sidebar = null;
        this.handleResize = this.handleResize.bind(this);
    }

    async init() {
        try {
            const container = document.getElementById('game-container');
            if (!container) {
                throw new Error('Game container not found');
            }

            this.canvas = document.createElement('canvas');
            this.canvas.className = 'office-canvas';
            container.replaceChildren(this.canvas);

            this.sidebar = new Sidebar();
            this.scene = new CanvasOfficeScene(this.canvas);
            this.scene.onStateUpdated = (state) => {
                if (this.sidebar) {
                    this.sidebar.updateState(state);
                }
            };
            this.scene.onActorSelected = ({ teamName, agentName }) => {
                if (this.sidebar && typeof this.sidebar.focusAgent === 'function') {
                    this.sidebar.focusAgent(teamName, agentName);
                }
            };

            await this.scene.init();
            window.addEventListener('resize', this.handleResize);
            this.handleResize();
            console.log('Canvas office scene initialized successfully');
        } catch (error) {
            console.error('Game initialization failed:', error);
            this.renderError(error);
            throw error;
        }
    }

    handleResize() {
        if (!this.scene) {
            return;
        }
        this.scene.handleResize();
    }

    renderError(error) {
        const container = document.getElementById('game-container');
        if (!container) {
            return;
        }

        const message = error instanceof Error ? error.message : String(error);
        container.innerHTML = `
            <div style="display:flex;align-items:center;justify-content:center;height:100%;padding:24px;box-sizing:border-box;">
                <div style="max-width:560px;background:#fff3f2;border:1px solid #f1b5b0;border-radius:12px;padding:20px 24px;color:#7f1d1d;box-shadow:0 4px 16px rgba(0,0,0,0.08);font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;">
                    <div style="font-size:18px;font-weight:700;margin-bottom:8px;">办公场景初始化失败</div>
                    <div style="font-size:14px;line-height:1.6;">${message}</div>
                    <div style="font-size:13px;color:#991b1b;margin-top:12px;">可先切回上方“监控面板”继续使用。</div>
                </div>
            </div>
        `;
    }

    destroy() {
        window.removeEventListener('resize', this.handleResize);
        if (this.scene) {
            this.scene.destroy();
            this.scene = null;
        }
        if (this.sidebar) {
            this.sidebar.destroy();
            this.sidebar = null;
        }
        if (this.canvas) {
            this.canvas.remove();
            this.canvas = null;
        }
    }
}

window.AgentMonitorGame = new Game();

document.addEventListener('DOMContentLoaded', async () => {
    await window.AgentMonitorGame.init();
});
