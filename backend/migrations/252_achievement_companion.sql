-- Account-scoped optional entertainment preference; independent of cash and VIP rules.
CREATE TABLE achievement_preferences (
 user_id bigint PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
 zodiac text NOT NULL DEFAULT '' CHECK (zodiac IN ('','aries','taurus','gemini','cancer','leo','virgo','libra','scorpio','sagittarius','capricorn','aquarius','pisces')),
 updated_at timestamptz NOT NULL DEFAULT clock_timestamp()
);

UPDATE achievement_catalog SET description='日晷轻转，晨昏相续。一百日的如约而至，将平凡的坚持，刻成时光里值得珍藏的印记。' WHERE key='S01';
UPDATE achievement_catalog SET description='日历渐厚，脚步未歇。把一个个寻常晨昏连成路，让每一次相逢，都有继续前行的回响。' WHERE key='S02';
UPDATE achievement_catalog SET description='春生夏长，秋收冬藏。日晷走过四季，把一整年的如约相见，珍藏在岁序的圆环里。' WHERE key='S03';
UPDATE achievement_catalog SET description='长夜初明，一点星火应念而生。让最初的好奇，照亮思考尚未抵达的远方。' WHERE key='T01';
UPDATE achievement_catalog SET description='微光渐聚，星芒初绽。那些不曾停下的思索，正把遥远的可能，照成眼前的光。' WHERE key='T02';
UPDATE achievement_catalog SET description='星辰循迹，思绪成轨。每一次向未知伸出的探问，都在浩瀚夜空中，留下清晰的来路。' WHERE key='T03';
UPDATE achievement_catalog SET description='千点微光，汇作星河。让散落的灵感彼此相逢，沿着思考的河床，奔向更辽阔的天地。' WHERE key='T04';
UPDATE achievement_catalog SET description='银河入海，群星无涯。在一次次探索之间，拓宽认知的岸线，收藏属于自己的浩瀚。' WHERE key='T05';
UPDATE achievement_catalog SET description='群星为钥，远方为门。当漫长的探索照亮门扉，新的天地，也在此刻缓缓展开。' WHERE key='T06';
UPDATE achievement_catalog SET description='信任有形，铭记于金。将最初的相伴刻入纹章，让每一份支持，都有值得珍藏的回声。' WHERE key='R01';
UPDATE achievement_catalog SET description='金色渐深，岁月成印。长久的信任，不止于一次相逢，而在同行的时光里，愈见温厚。' WHERE key='R02';
UPDATE achievement_catalog SET description='万千心意，凝作典藏。把一路相伴的信任，铸成沉静而厚重的荣光，珍藏于岁月深处。' WHERE key='R03';
UPDATE achievement_catalog SET description='书页初启，星图渐明。循着好奇点亮第一处坐标，让陌生的世界，有了可以出发的方向。' WHERE key='A-K01';
UPDATE achievement_catalog SET description='经纬相连，纷繁有序。在文字与符号之间读懂彼此，让每一道讯息，都能抵达应去的地方。' WHERE key='A-K02';
UPDATE achievement_catalog SET description='群星入册，所学成藏。把散落的答案收进书页，让曾经的疑问，成为照亮下一程的灯。' WHERE key='A-K03';
UPDATE achievement_catalog SET description='接通第一束微光，远方就有了回应。循着清晰的航线启程，让每一次探索，都从稳稳的抵达开始。' WHERE key='A-X01';
UPDATE achievement_catalog SET description='进退有节，行止有度。让每一分投入各得其所，在从容的节奏里，走向更长久的远方。' WHERE key='A-X02';
UPDATE achievement_catalog SET description='循迹解惑，拨雾见明。于纷繁的线索中找到秩序，让每一次迎刃而解，都化作下一程的从容。' WHERE key='A-X03';
UPDATE achievement_catalog SET description='罗盘初定，足迹初成。将沿途的发现汇成一枚印记，为这段探索，留下一处温柔的坐标。' WHERE key='A-C01';
UPDATE achievement_catalog SET description='翻过一页，山海又新。让不同篇章里的相逢，连成彼此呼应的故事，把同行写进更远的风景。' WHERE key='A-C02';
UPDATE achievement_catalog SET description='山海入卷，万象成藏。当散落的篇章汇成完整星图，回望来路，每一程都有自己的光芒。' WHERE key='A-C03';
