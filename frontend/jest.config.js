/** @type {import('ts-jest').JestConfigWithTsJest} */
module.exports = {
  //1.- Use the Next.js testing preset to include base transforms and environment settings.
  preset: 'ts-jest',
  //2.- Execute tests in a browser-like environment so DOM APIs are available.
  testEnvironment: 'jsdom',
  //3.- Configure setup files to register helpful matchers globally.
  setupFilesAfterEnv: ['<rootDir>/jest.setup.ts'],
  //4.- Limit the test discovery roots to the project workspace.
  roots: ['<rootDir>'],
  //5.- Ignore built assets and node modules during the test run.
  testPathIgnorePatterns: ['/node_modules/', '/.next/'],
  //6.- Support the @ alias that mirrors the Next.js TypeScript configuration.
  moduleNameMapper: {
    '^@/(.*)$': '<rootDir>/$1',
  },
};
