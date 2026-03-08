import { DrawFunctions } from './DrawFunctions.js';

export class ResourceManager {
    constructor(scene) {
        this.scene = scene;
        this.config = null;
        this.loadedImages = new Map();
        this.drawFunctions = new Map();

        // 注册绘制函数
        this.registerDrawFunction('drawAgent', DrawFunctions.drawAgent);
        this.registerDrawFunction('drawOffice', DrawFunctions.drawOffice);
        this.registerDrawFunction('drawFacility', DrawFunctions.drawFacility);
    }

    async init() {
        // 加载配置文件
        try {
            const response = await fetch('js/game/config/assets.json');
            this.config = await response.json();
            console.log('Assets config loaded:', this.config);
        } catch (error) {
            console.error('Failed to load assets config:', error);
            this.config = { agents: {}, rooms: {}, facilities: {} };
        }

        // 尝试加载所有图片
        await this.loadAllImages();
    }

    async loadAllImages() {
        const promises = [];

        for (const [category, items] of Object.entries(this.config)) {
            for (const [key, asset] of Object.entries(items)) {
                if (asset.type === 'image') {
                    const assetKey = `${category}_${key}`;
                    promises.push(this.tryLoadImage(asset.path, assetKey));
                }
            }
        }

        await Promise.all(promises);
        console.log('Image loading complete. Loaded:', this.loadedImages.size);
    }

    async tryLoadImage(path, key) {
        return new Promise((resolve) => {
            this.scene.load.image(key, path);
            this.scene.load.once('filecomplete-image-' + key, () => {
                this.loadedImages.set(key, true);
                console.log(`Image loaded: ${key}`);
                resolve();
            });
            this.scene.load.once('loaderror', () => {
                this.loadedImages.set(key, false);
                console.log(`Image not found, using fallback: ${key}`);
                resolve();
            });
            this.scene.load.start();
        });
    }

    getAsset(category, key) {
        const assetKey = `${category}_${key}`;

        if (this.loadedImages.get(assetKey) === true) {
            return { type: 'image', key: assetKey };
        } else {
            const fallbackName = this.config[category]?.[key]?.fallback || 'drawAgent';
            const func = this.drawFunctions.get(fallbackName);
            return { type: 'draw', func: func };
        }
    }

    registerDrawFunction(name, func) {
        this.drawFunctions.set(name, func);
    }

    hasImage(category, key) {
        const assetKey = `${category}_${key}`;
        return this.loadedImages.get(assetKey) === true;
    }
}
